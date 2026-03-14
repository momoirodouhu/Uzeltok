package handler

import (
	"fmt"
	"hash/crc32"
	"math"
	"net/http"

	"uzeltok/internal/model"
	"uzeltok/internal/store"
)

// handleIndex は / を処理する公開ハンドラです。
func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=2592000") // 30 days
	if err := h.view.Render(w, "index.gohtml", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleLinkDetail は /{id} を処理する公開ハンドラです。
func (h *Handler) handleLinkDetail(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("id")
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

	if l.IsExpired() {
		h.notFound(w, r)
		return
	}

	etag := generateETag(l)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, no-cache")

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h.renderLinkPage(w, r, l)
}

// handlePublicUpload は Drop リンクへのファイルアップロードを処理します。
func (h *Handler) handlePublicUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	uuid := r.PathValue("id")
	l, err := h.fetchLink(uuid)
	if err != nil {
		if err == store.ErrNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if l.IsExpired() {
		h.notFound(w, r)
		return
	}

	if l.Metadata.Type != model.TypeDrop {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if status, msg := h.saveMultipartUpload(w, r, l.ID); status != 0 {
		http.Error(w, msg, status)
		return
	}

	// GET /{id} にリダイレクトして完了を示す
	http.Redirect(w, r, "/"+l.ID+"?success=upload", http.StatusSeeOther)
}

// handlePublicDownload は /{id}/files/{filename} を処理する公開ハンドラです。
func (h *Handler) handlePublicDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, no-cache")

	uuid := r.PathValue("id")
	l, err := h.fetchLink(uuid)
	if err != nil {
		if err == store.ErrNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if l.IsExpired() {
		h.notFound(w, r)
		return
	}

	filename := r.PathValue("filename")
	if filename == "" {
		h.notFound(w, r)
		return
	}

	if l.Metadata.Type != model.TypeShare {
		h.notFound(w, r)
		return
	}

	h.serveFile(w, r, l, filename)
}

// renderLinkPage はリンクの種類に応じたテンプレートを描画します。
func (h *Handler) renderLinkPage(w http.ResponseWriter, r *http.Request, l *model.Link) {
	tmpl := "share.gohtml"
	if l.Metadata.Type == model.TypeDrop {
		tmpl = "drop.gohtml"
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	data := struct {
		*model.Link
		Host           string
		BaseURL        string
		MaxUploadLabel string
	}{
		Link:           l,
		Host:           r.Host,
		BaseURL:        fmt.Sprintf("%s://%s", scheme, r.Host),
		MaxUploadLabel: humanizeBytesIEC(h.cfg.MaxUploadBytes),
	}

	if err := h.view.Render(w, tmpl, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func humanizeBytesIEC(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}

	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	value := float64(n)
	unitIdx := -1
	for value >= 1024 && unitIdx < len(units)-1 {
		value /= 1024
		unitIdx++
	}

	if unitIdx >= 0 && math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%.0f %s", value, units[unitIdx])
	}
	return fmt.Sprintf("%.1f %s", value, units[unitIdx])
}

// generateETag はリンク情報とそのファイルリストからETagを生成します。
func generateETag(l *model.Link) string {
	h := crc32.NewIEEE()
	fmt.Fprintf(h, "link:%s\n", l.ID)
	if !l.Metadata.ExpiresAt.IsZero() {
		fmt.Fprintf(h, "expires:%d\n", l.Metadata.ExpiresAt.Unix())
	}
	for _, f := range l.Files {
		fmt.Fprintf(h, "file:%s:%d:%d\n", f.Name, f.Size, f.Timestamp.UnixNano())
	}
	return fmt.Sprintf(`"%08x"`, h.Sum32())
}
