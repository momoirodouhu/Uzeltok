package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"uzeltok/internal/model"
	"uzeltok/internal/store"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/tus/tusd/v2/pkg/filestore"
	tushandler "github.com/tus/tusd/v2/pkg/handler"
)

const (
	tusMetaLinkID = "link_id"
	tusMetaScope  = "scope"
	tusMetaFile   = "filename"
	tusScopeAdmin = "admin"
	tusScopeDrop  = "drop"
	tusIDAlphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

type tusRouteTarget struct {
	linkID   string
	scope    string
	uploadID string
}

func (h *Handler) initTus() error {
	tusDir := filepath.Join(h.links.BaseDir(), "_tus")
	if err := os.MkdirAll(tusDir, 0o755); err != nil {
		return fmt.Errorf("failed to create tus dir: %w", err)
	}

	if h.cfg.TusIncompleteTTL > 0 {
		if _, err := cleanupStaleTusUploads(tusDir, h.cfg.TusIncompleteTTL); err != nil {
			return fmt.Errorf("failed to cleanup stale tus uploads: %w", err)
		}
	}

	fileStore := filestore.New(tusDir)
	composer := tushandler.NewStoreComposer()
	fileStore.UseIn(composer)

	unrouted, err := tushandler.NewUnroutedHandler(tushandler.Config{
		BasePath:                  "/",
		StoreComposer:             composer,
		MaxSize:                   h.cfg.MaxUploadBytes,
		DisableDownload:           true,
		PreUploadCreateCallback:   h.onTusPreCreate,
		PreFinishResponseCallback: h.onTusPreFinish,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize tus handler: %w", err)
	}

	h.tusHTTP = unrouted.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			unrouted.PostFile(w, r)
		case http.MethodHead:
			unrouted.HeadFile(w, r)
		case http.MethodPatch:
			unrouted.PatchFile(w, r)
		case http.MethodDelete:
			unrouted.DelFile(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	return nil
}

func cleanupStaleTusUploads(tusDir string, ttl time.Duration) (int, error) {
	if ttl <= 0 {
		return 0, nil
	}

	cutoff := time.Now().Add(-ttl)
	staleFiles := make([]string, 0, 16)

	err := filepath.WalkDir(tusDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if info.ModTime().Before(cutoff) {
			staleFiles = append(staleFiles, path)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	deleted := 0
	for _, p := range staleFiles {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, err
		}
		deleted++
		removeEmptyTusParents(filepath.Dir(p))
	}

	return deleted, nil
}

func (h *Handler) onTusPreCreate(hook tushandler.HookEvent) (tushandler.HTTPResponse, tushandler.FileInfoChanges, error) {
	target, err := h.validateTusRoute(hook.HTTPRequest.URI, true)
	if err != nil {
		return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, err
	}

	filename := sanitizeTusFilename(hook.Upload.MetaData[tusMetaFile], hook.Upload.ID)
	uploadID, err := newTusUploadID(target.linkID)
	if err != nil {
		return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_UPLOAD_INIT", "failed to initialize upload", http.StatusInternalServerError)
	}
	_, err = h.links.FilePath(target.linkID, filename)
	if err != nil {
		return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{}, tushandler.NewError("ERR_INVALID_UPLOAD", "invalid upload target", http.StatusBadRequest)
	}

	metadata := tushandler.MetaData{
		tusMetaLinkID: target.linkID,
		tusMetaScope:  target.scope,
		tusMetaFile:   filename,
	}
	if filetype := strings.TrimSpace(hook.Upload.MetaData["filetype"]); filetype != "" {
		metadata["filetype"] = filetype
	}

	return tushandler.HTTPResponse{}, tushandler.FileInfoChanges{
		ID:       uploadID,
		MetaData: metadata,
	}, nil
}

func (h *Handler) onTusPreFinish(hook tushandler.HookEvent) (tushandler.HTTPResponse, error) {
	linkID := strings.TrimSpace(hook.Upload.MetaData[tusMetaLinkID])
	filename := strings.TrimSpace(hook.Upload.MetaData[tusMetaFile])
	if linkID == "" || filename == "" {
		return tushandler.HTTPResponse{}, tushandler.NewError("ERR_UPLOAD_INVALID", "invalid upload metadata", http.StatusBadRequest)
	}

	srcPath := strings.TrimSpace(hook.Upload.Storage[filestore.StorageKeyPath])
	if srcPath == "" {
		return tushandler.HTTPResponse{}, tushandler.NewError("ERR_UPLOAD_INVALID", "missing upload storage path", http.StatusInternalServerError)
	}

	if err := h.uploads.PersistTusUpload(linkID, filename, srcPath); err != nil {
		return tushandler.HTTPResponse{}, tushandler.NewError("ERR_UPLOAD_SAVE", "failed to persist uploaded file", http.StatusInternalServerError)
	}

	return tushandler.HTTPResponse{}, nil
}

func removeEmptyTusParents(dir string) {
	for {
		if dir == "" || dir == "." || dir == string(filepath.Separator) {
			return
		}
		if filepath.Base(dir) == "_tus" {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func (h *Handler) validateTusRoute(rawURI string, requireDrop bool) (tusRouteTarget, error) {
	target, err := parseTusRoute(rawURI)
	if err != nil {
		return tusRouteTarget{}, err
	}

	md, err := h.links.GetLinkMetadata(target.linkID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return tusRouteTarget{}, tushandler.NewError("ERR_UPLOAD_NOT_FOUND", "link not found", http.StatusNotFound)
		}
		return tusRouteTarget{}, tushandler.NewError("ERR_UPLOAD_CHECK_FAILED", "failed to load link", http.StatusInternalServerError)
	}

	if target.scope == tusScopeDrop && !md.ExpiresAt.IsZero() && time.Now().After(md.ExpiresAt) {
		return tusRouteTarget{}, tushandler.NewError("ERR_UPLOAD_EXPIRED", "link expired", http.StatusNotFound)
	}
	if requireDrop && target.scope == tusScopeDrop && md.Type != model.TypeDrop {
		return tusRouteTarget{}, tushandler.NewError("ERR_UPLOAD_NOT_ALLOWED", "upload is only allowed for drop links", http.StatusMethodNotAllowed)
	}
	if target.uploadID != "" && strings.Contains(target.uploadID, "/") {
		return tusRouteTarget{}, tushandler.NewError("ERR_UPLOAD_NOT_FOUND", "upload not found", http.StatusNotFound)
	}

	return target, nil
}

func parseTusRoute(rawURI string) (tusRouteTarget, error) {
	requestPath := rawURI
	if u, err := url.ParseRequestURI(rawURI); err == nil {
		requestPath = u.Path
	}

	if strings.HasPrefix(requestPath, "/admin/links/") {
		rest := strings.TrimPrefix(requestPath, "/admin/links/")
		linkID, tail, ok := strings.Cut(rest, "/")
		if !ok || linkID == "" {
			return tusRouteTarget{}, tushandler.NewError("ERR_INVALID_UPLOAD", "invalid upload target", http.StatusBadRequest)
		}
		if tail == "tus" {
			return tusRouteTarget{linkID: linkID, scope: tusScopeAdmin}, nil
		}
		if strings.HasPrefix(tail, "tus/") {
			uploadID := strings.TrimPrefix(tail, "tus/")
			if uploadID == "" {
				return tusRouteTarget{}, tushandler.NewError("ERR_INVALID_UPLOAD", "invalid upload target", http.StatusBadRequest)
			}
			return tusRouteTarget{linkID: linkID, scope: tusScopeAdmin, uploadID: uploadID}, nil
		}
		return tusRouteTarget{}, tushandler.NewError("ERR_INVALID_UPLOAD", "invalid upload target", http.StatusBadRequest)
	}

	rest := strings.TrimPrefix(requestPath, "/")
	linkID, tail, ok := strings.Cut(rest, "/")
	if !ok || linkID == "" {
		return tusRouteTarget{}, tushandler.NewError("ERR_INVALID_UPLOAD", "invalid upload target", http.StatusBadRequest)
	}
	if tail == "tus" {
		return tusRouteTarget{linkID: linkID, scope: tusScopeDrop}, nil
	}
	if strings.HasPrefix(tail, "tus/") {
		uploadID := strings.TrimPrefix(tail, "tus/")
		if uploadID == "" {
			return tusRouteTarget{}, tushandler.NewError("ERR_INVALID_UPLOAD", "invalid upload target", http.StatusBadRequest)
		}
		return tusRouteTarget{linkID: linkID, scope: tusScopeDrop, uploadID: uploadID}, nil
	}

	return tusRouteTarget{}, tushandler.NewError("ERR_INVALID_UPLOAD", "invalid upload target", http.StatusBadRequest)
}

func sanitizeTusFilename(rawName, fallback string) string {
	name := strings.TrimSpace(strings.ReplaceAll(rawName, "\x00", ""))
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fallback
	}
	return name
}

func newTusUploadID(linkID string) (string, error) {
	suffix, err := gonanoid.Generate(tusIDAlphabet, 24)
	if err != nil {
		return "", err
	}
	return linkID + "/" + suffix, nil
}

func rewriteTusLocation(location, routeBase string) string {
	if location == "" {
		return ""
	}

	uploadID := strings.TrimPrefix(location, "/")
	if parsed, err := url.Parse(location); err == nil {
		uploadID = strings.TrimPrefix(parsed.Path, "/")
	}
	if _, suffix, ok := strings.Cut(uploadID, "/"); ok && suffix != "" {
		uploadID = suffix
	}

	// Return a relative Location path so the client resolves scheme/host from current origin.
	return strings.TrimRight(routeBase, "/") + "/" + strings.TrimPrefix(uploadID, "/")
}

func toTusStorageUploadID(linkID, uploadID string) string {
	if uploadID == "" {
		return ""
	}
	return linkID + "/" + uploadID
}

func cloneTusRequest(r *http.Request, uploadID string) *http.Request {
	clone := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = "/"
	urlCopy.RawPath = ""
	if uploadID != "" {
		urlCopy.Path = "/" + uploadID
	}
	clone.URL = &urlCopy
	return clone
}

type tusLocationRewriter struct {
	http.ResponseWriter
	routeBase   string
	wroteHeader bool
}

func (w *tusLocationRewriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *tusLocationRewriter) WriteHeader(statusCode int) {
	if location := w.Header().Get("Location"); location != "" {
		w.Header().Set("Location", rewriteTusLocation(location, w.routeBase))
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *tusLocationRewriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (h *Handler) serveTus(w http.ResponseWriter, r *http.Request, routeBase, uploadID string) {
	rewriter := &tusLocationRewriter{ResponseWriter: w, routeBase: routeBase}
	h.tusHTTP.ServeHTTP(rewriter, cloneTusRequest(r, uploadID))
}

func tusStatusCode(err error) int {
	var tusErr tushandler.Error
	if errors.As(err, &tusErr) {
		return tusErr.HTTPResponse.StatusCode
	}
	return http.StatusBadRequest
}

func (h *Handler) handleAdminTus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	target, err := h.validateTusRoute(r.RequestURI, false)
	if err != nil {
		if tusStatusCode(err) == http.StatusNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), tusStatusCode(err))
		return
	}

	routeBase := "/admin/links/" + target.linkID + "/tus"
	h.serveTus(w, r, routeBase, toTusStorageUploadID(target.linkID, target.uploadID))
}

func (h *Handler) handlePublicTus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	target, err := h.validateTusRoute(r.RequestURI, true)
	if err != nil {
		if tusStatusCode(err) == http.StatusNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), tusStatusCode(err))
		return
	}

	routeBase := "/" + target.linkID + "/tus"
	h.serveTus(w, r, routeBase, toTusStorageUploadID(target.linkID, target.uploadID))
}
