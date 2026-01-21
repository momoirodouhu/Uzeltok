package handler

import (
    "fmt"
    "net/http"
    "strings"

    "uzeltok/internal/store"
)

type Handler struct {
    store *store.LinkStore
}

func NewHandler(s *store.LinkStore) *Handler {
    return &Handler{store: s}
}

// RegisterRoutes は http.ServeMux に必要なパスを登録します。
// 動的セグメントの解析はこのハンドラ内で行います（標準 ServeMux を使用）。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("/share/", h.handleShare)
    mux.HandleFunc("/drop/", h.handleDrop)
}

func (h *Handler) handleShare(w http.ResponseWriter, r *http.Request) {
    // path: /share/<uuid> or /share/<uuid>/<filename>
    p := strings.TrimPrefix(r.URL.Path, "/share/")
    if p == "" {
        http.NotFound(w, r)
        return
    }
    parts := strings.SplitN(p, "/", 2)
    uuid := parts[0]
    if uuid == "" {
        http.NotFound(w, r)
        return
    }
    if len(parts) == 1 || parts[1] == "" {
        // メタやファイル一覧表示用エンドポイント（未実装）
        http.Error(w, fmt.Sprintf("share info for %s: not implemented", uuid), http.StatusNotImplemented)
        return
    }
    filename := parts[1]
    // ダウンロード用エンドポイント（未実装）
    http.Error(w, fmt.Sprintf("share download %s / %s: not implemented", uuid, filename), http.StatusNotImplemented)
}

func (h *Handler) handleDrop(w http.ResponseWriter, r *http.Request) {
    // path: /drop/<uuid> or /drop/<uuid>/<filename>
    p := strings.TrimPrefix(r.URL.Path, "/drop/")
    if p == "" {
        http.NotFound(w, r)
        return
    }
    parts := strings.SplitN(p, "/", 2)
    uuid := parts[0]
    if uuid == "" {
        http.NotFound(w, r)
        return
    }
    if len(parts) == 1 || parts[1] == "" {
        // ドロップの情報取得用（未実装）
        http.Error(w, fmt.Sprintf("drop info for %s: not implemented", uuid), http.StatusNotImplemented)
        return
    }
    filename := parts[1]
    if r.Method == http.MethodPost {
        // アップロード用エンドポイント（未実装）
        http.Error(w, fmt.Sprintf("drop upload %s / %s: not implemented", uuid, filename), http.StatusNotImplemented)
        return
    }
    // その他（未実装）
    http.Error(w, fmt.Sprintf("drop endpoint %s / %s: not implemented", uuid, filename), http.StatusNotImplemented)
}
