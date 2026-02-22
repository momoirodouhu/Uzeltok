package handler

import (
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

type Handler struct {
	store     *store.LinkStore
	view      *web.Provider
	adminPass string
}

func NewHandler(s *store.LinkStore, v *web.Provider, adminPass string) *Handler {
	return &Handler{store: s, view: v, adminPass: adminPass}
}

// RegisterRoutes は http.ServeMux に必要なパスを登録します。
// 動的セグメントの解析はこのハンドラ内で行います（標準 ServeMux を使用）。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return adminAuth(next, h.adminPass)
	}

	// Admin routes (Basic Auth)
	mux.HandleFunc("/admin", auth(h.handleAdmin))
	mux.HandleFunc("/admin/", auth(h.handleAdminSub))

	// Public routes
	mux.HandleFunc("/", h.handleLink)
}

// --- Admin handlers ---

// handleAdmin は全リンクの一覧を表示する管理画面を返します。
func (h *Handler) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	links, err := h.store.ListLinks()
	if err != nil {
		http.Error(w, "Failed to list links", http.StatusInternalServerError)
		return
	}

	// 新しい順に並び替え
	for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
		links[i], links[j] = links[j], links[i]
	}

	err = h.view.Render(w, "admin.gohtml", links)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAdminSub は /admin/{id} と /admin/{id}/{filename} と /admin/{id}/upload を処理します。
func (h *Handler) handleAdminSub(w http.ResponseWriter, r *http.Request) {
	// path: /admin/{id} or /admin/{id}/{filename} or /admin/{id}/upload
	p := strings.TrimPrefix(r.URL.Path, "/admin/")
	if p == "" {
		h.handleAdmin(w, r)
		return
	}

	parts := strings.SplitN(p, "/", 2)
	uuid := parts[0]
	if uuid == "" {
		h.notFound(w, r)
		return
	}

	l, err := h.fetchLink(uuid)
	if err != nil {
		if err == store.ErrNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// /admin/{id}
	if len(parts) == 1 || parts[1] == "" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		err = h.view.Render(w, "admin_link.gohtml", l)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	sub := parts[1]

	// /admin/{id}/upload
	if sub == "upload" {
		h.handleAdminUpload(w, r, l)
		return
	}

	// /admin/{id}/delete — ファイル削除 (POST with filename in form)
	if sub == "delete" {
		h.handleAdminDelete(w, r, l)
		return
	}

	// /admin/{id}/{filename} — ファイルダウンロード
	h.serveFile(w, r, l, sub)
}

// handleAdminUpload は管理者によるファイルアップロードを処理します。
func (h *Handler) handleAdminUpload(w http.ResponseWriter, r *http.Request, l *model.Link) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 32MB max
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files provided", http.StatusBadRequest)
		return
	}

	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		err = h.store.SaveFile(l.ID, fh.Filename, f)
		f.Close()
		if err != nil {
			http.Error(w, "failed to save file: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// アップロード後にリンク詳細ページへリダイレクト
	http.Redirect(w, r, "/admin/"+l.ID, http.StatusSeeOther)
}

// handleAdminDelete は管理者によるファイル削除を処理します。
func (h *Handler) handleAdminDelete(w http.ResponseWriter, r *http.Request, l *model.Link) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteFile(l.ID, filename); err != nil {
		if err == store.ErrNotFound {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/"+l.ID, http.StatusSeeOther)
}

// --- Public handlers ---

// fetchLink はストアからリンクを取得します。呼び出し元でエラーハンドリングを行います。
func (h *Handler) fetchLink(uuid string) (*model.Link, error) {
	return h.store.GetLink(uuid)
}

// writeLinkInfo はリンクのメタ情報とファイル一覧をテキストで書き出します。
func (h *Handler) writeLinkInfo(w http.ResponseWriter, l *model.Link) {
	fmt.Fprintf(w, "ID: %s\n", l.ID)
	fmt.Fprintf(w, "Type: %s\n", l.Metadata.Type)
	fmt.Fprintf(w, "CreatedAt: %s\n", l.Metadata.CreatedAt.Format(time.RFC3339))
	if !l.Metadata.ExpiresAt.IsZero() {
		fmt.Fprintf(w, "ExpiresAt: %s\n", l.Metadata.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Fprintln(w, "Files:")
	if len(l.Files) == 0 {
		fmt.Fprintln(w, "  (no files)")
	} else {
		for _, f := range l.Files {
			fmt.Fprintf(w, "  - %s (%d bytes)\n", f.Name, f.Size)
		}
	}
}

// serveFile は指定されたリンクとファイル名に対してファイルレスポンスを返します。
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, l *model.Link, filename string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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

	if rs, ok := rc.(io.ReadSeeker); ok {
		var modtime time.Time
		if f, ok := rc.(*os.File); ok {
			if st, err := f.Stat(); err == nil {
				modtime = st.ModTime()
			}
		}
		http.ServeContent(w, r, filename, modtime, rs)
		return
	}

	data, err := io.ReadAll(rc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, filename, time.Time{}, bytes.NewReader(data))
}

// handleLink は `/<uuid>` と `/<uuid>/<filename>` の両方を処理する共通ハンドラです。
// パスの先頭セグメントを気にせず UUID と省略可能なファイル名を処理します。
func (h *Handler) handleLink(w http.ResponseWriter, r *http.Request) {
	// path: /<uuid> or /<uuid>/<filename>
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		err := h.view.Render(w, "index.gohtml", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	parts := strings.SplitN(p, "/", 2)
	uuid := parts[0]
	if uuid == "" {
		h.notFound(w, r)
		return
	}
	if len(parts) == 1 || parts[1] == "" {
		// Link 情報をテキストで返す
		l, err := h.fetchLink(uuid)
		if err != nil {
			if err == store.ErrNotFound {
				h.notFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl := "share.gohtml"
		if l.Metadata.Type == model.TypeDrop {
			tmpl = "drop.gohtml"
		}

		err = h.view.Render(w, tmpl, l)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	// filename が指定されている場合はファイルを返す
	filename := parts[1]
	l, err := h.fetchLink(uuid)
	if err != nil {
		if err == store.ErrNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.serveFile(w, r, l, filename)
}

// notFound は 404 Not Found ページを返します。
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	err := h.view.Render(w, "404.gohtml", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
