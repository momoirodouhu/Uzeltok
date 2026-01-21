package handler

import (
    "fmt"
    "net/http"
    "strings"
    "time"

    "uzeltok/internal/model"
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
        // Link 情報をテキストで返す
        l, err := h.store.GetLink(model.TypeShare, uuid)
        if err != nil {
            if err == store.ErrNotFound {
                http.NotFound(w, r)
                return
            }
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        fmt.Fprintf(w, "ID: %s\n", l.ID)
        fmt.Fprintf(w, "Type: %s\n", l.Type)
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
        return
    }
    // filename が指定されている場合は未実装のままにする
    http.Error(w, fmt.Sprintf("share download %s / %s: not implemented", uuid, parts[1]), http.StatusNotImplemented)
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
        // Link 情報をテキストで返す
        l, err := h.store.GetLink(model.TypeDrop, uuid)
        if err != nil {
            if err == store.ErrNotFound {
                http.NotFound(w, r)
                return
            }
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        fmt.Fprintf(w, "ID: %s\n", l.ID)
        fmt.Fprintf(w, "Type: %s\n", l.Type)
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
        return
    }
    // filename が指定されている場合は未実装のままにする
    filename := parts[1]
    if r.Method == http.MethodPost {
        http.Error(w, fmt.Sprintf("drop upload %s / %s: not implemented", uuid, filename), http.StatusNotImplemented)
        return
    }
    http.Error(w, fmt.Sprintf("drop endpoint %s / %s: not implemented", uuid, filename), http.StatusNotImplemented)
}
