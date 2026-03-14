package handler

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"uzeltok/internal/model"
	"uzeltok/internal/service"
	"uzeltok/internal/store"
	"uzeltok/internal/web"
)

// Handler は全ての HTTP ハンドラが共有する依存を保持します。
type Handler struct {
	links          *service.LinkService
	uploads        *service.UploadService
	fileDelivery   *fileDelivery
	view           *web.Provider
	adminPass      string
	maxUploadBytes int64
	csrfSecret     [32]byte
	tusHTTP        http.Handler
}

// NewHandler は新しい Handler を生成します。
func NewHandler(s *store.LinkStore, v *web.Provider, adminPass string, maxUploadBytes int64) (*Handler, error) {
	links := service.NewLinkService(s)
	h := &Handler{
		links:          links,
		uploads:        service.NewUploadService(links),
		fileDelivery:   newFileDelivery(links),
		view:           v,
		adminPass:      adminPass,
		maxUploadBytes: maxUploadBytes,
	}
	if _, err := rand.Read(h.csrfSecret[:]); err != nil {
		return nil, fmt.Errorf("failed to initialize CSRF secret: %w", err)
	}

	if err := h.initTus(); err != nil {
		return nil, err
	}

	return h, nil
}

// RegisterRoutes は http.ServeMux に必要なパスを登録します。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	h.registerAdminRoutes(mux)
	h.registerPublicRoutes(mux)
}

// Handler はセキュリティヘッダー付きの http.Handler を返します。
func (h *Handler) Handler(mux *http.ServeMux) http.Handler {
	h.RegisterRoutes(mux)
	return securityHeaders(mux)
}

// --- 共通ヘルパー ---

// fetchLink はストアからリンクを取得します。
func (h *Handler) fetchLink(uuid string) (*model.Link, error) {
	return h.links.GetLink(uuid)
}

// serveFile は指定されたリンクとファイル名に対してファイルレスポンスを返します。
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, l *model.Link, filename string) {
	h.fileDelivery.Serve(w, r, l, filename, h.notFound)
}

// notFound は 404 Not Found ページを返します。
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	if err := h.view.Render(w, "404.gohtml", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
