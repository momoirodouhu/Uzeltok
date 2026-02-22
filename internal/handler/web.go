package handler

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"time"

	"uzeltok/internal/model"
	"uzeltok/internal/store"
	"uzeltok/internal/web"
)

// Handler は全ての HTTP ハンドラが共有する依存を保持します。
type Handler struct {
	store     *store.LinkStore
	view      *web.Provider
	adminPass string
}

// NewHandler は新しい Handler を生成します。
func NewHandler(s *store.LinkStore, v *web.Provider, adminPass string) *Handler {
	return &Handler{store: s, view: v, adminPass: adminPass}
}

// RegisterRoutes は http.ServeMux に必要なパスを登録します。
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

// --- 共通ヘルパー ---

// fetchLink はストアからリンクを取得します。
func (h *Handler) fetchLink(uuid string) (*model.Link, error) {
	return h.store.GetLink(uuid)
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

// notFound は 404 Not Found ページを返します。
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	if err := h.view.Render(w, "404.gohtml", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
