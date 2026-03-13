package handler

import (
	"crypto/rand"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"uzeltok/internal/model"
	"uzeltok/internal/store"
	"uzeltok/internal/web"
)

// Handler は全ての HTTP ハンドラが共有する依存を保持します。
type Handler struct {
	store          *store.LinkStore
	view           *web.Provider
	adminPass      string
	maxUploadBytes int64
	csrfSecret     [32]byte
}

// NewHandler は新しい Handler を生成します。
func NewHandler(s *store.LinkStore, v *web.Provider, adminPass string, maxUploadBytes int64) *Handler {
	h := &Handler{store: s, view: v, adminPass: adminPass, maxUploadBytes: maxUploadBytes}
	if _, err := rand.Read(h.csrfSecret[:]); err != nil {
		panic("failed to initialize CSRF secret")
	}
	return h
}

// RegisterRoutes は http.ServeMux に必要なパスを登録します。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return h.adminAuth(next)
	}

	// Method override middleware for HTML forms
	methodOverride := func(next http.Handler) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				if override := r.FormValue("_method"); override != "" {
					r.Method = strings.ToUpper(override)
				}
			}
			next.ServeHTTP(w, r)
		})
	}

	// Wrapper to apply method override to specific routes
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		return methodOverride(h).ServeHTTP
	}

	// Admin routes (Basic Auth + CSRF protection for mutations)
	mux.HandleFunc("GET /admin", auth(h.handleAdmin))
	mux.HandleFunc("POST /admin/links", auth(h.csrfProtect(wrap(h.handleAdminCreateLink))))
	mux.HandleFunc("GET /admin/links/{id}", auth(h.handleAdminDetail))
	mux.HandleFunc("POST /admin/links/{id}", auth(h.csrfProtect(wrap(h.handleAdminDeleteLink))))
	mux.HandleFunc("POST /admin/links/{id}/files", auth(h.csrfProtect(h.handleAdminUpload)))
	mux.HandleFunc("POST /admin/links/{id}/files/{filename}", auth(h.csrfProtect(wrap(h.handleAdminDeleteFile))))
	mux.HandleFunc("GET /admin/links/{id}/files/{filename}", auth(h.handleAdminDownloadFile))

	// Public routes
	mux.HandleFunc("GET /{$}", h.handleIndex)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
	})
	mux.HandleFunc("GET /{id}", h.handleLinkDetail)
	mux.HandleFunc("POST /{id}/files", h.handlePublicUpload)
	mux.HandleFunc("GET /{id}/files/{filename}", h.handlePublicDownload)
}

// Handler はセキュリティヘッダー付きの http.Handler を返します。
func (h *Handler) Handler(mux *http.ServeMux) http.Handler {
	h.RegisterRoutes(mux)
	return securityHeaders(mux)
}

// --- 共通ヘルパー ---

// fetchLink はストアからリンクを取得します。
func (h *Handler) fetchLink(uuid string) (*model.Link, error) {
	return h.store.GetLink(uuid)
}

// serveFile は指定されたリンクとファイル名に対してファイルレスポンスを返します。
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, l *model.Link, filename string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")

	rc, err := h.store.OpenFile(l, filename)
	if err != nil {
		if err == store.ErrNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rc.Close()

	var modtime time.Time
	var size int64

	fallbackModtime := time.Time{}
	for _, f := range l.Files {
		if f.Name == filename {
			fallbackModtime = f.Timestamp
			size = f.Size
			break
		}
	}

	if rs, ok := rc.(io.ReadSeeker); ok {
		if f, ok := rc.(*os.File); ok {
			if st, err := f.Stat(); err == nil {
				modtime = st.ModTime()
				size = st.Size()
			}
		}

		if modtime.IsZero() {
			modtime = fallbackModtime
		}

		w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, modtime.UnixNano(), size))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(filename)))

		http.ServeContent(w, r, filename, modtime, rs)
		return
	}

	data, err := io.ReadAll(rc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if size == 0 {
		size = int64(len(data))
	}
	modtime = fallbackModtime

	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, modtime.UnixNano(), size))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(filename)))

	http.ServeContent(w, r, filename, modtime, bytes.NewReader(data))
}

// notFound は 404 Not Found ページを返します。
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	if err := h.view.Render(w, "404.gohtml", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
