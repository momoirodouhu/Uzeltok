package handler

import (
	"net/http"

	"uzeltok/internal/model"
	"uzeltok/internal/store"
)

// handleIndex は / を処理する公開ハンドラです。
func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
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

	h.renderLinkPage(w, r, l)
}

// handlePublicUpload は Drop リンクへのファイルアップロードを処理します。
func (h *Handler) handlePublicUpload(w http.ResponseWriter, r *http.Request) {
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

	// GET /{id} にリダイレクトして完了を示す
	http.Redirect(w, r, "/"+l.ID+"?success=upload", http.StatusSeeOther)
}

// handlePublicDownload は /{id}/files/{filename} を処理する公開ハンドラです。
func (h *Handler) handlePublicDownload(w http.ResponseWriter, r *http.Request) {
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

	h.serveFile(w, r, l, filename)
}

// renderLinkPage はリンクの種類に応じたテンプレートを描画します。
func (h *Handler) renderLinkPage(w http.ResponseWriter, r *http.Request, l *model.Link) {
	tmpl := "share.gohtml"
	if l.Metadata.Type == model.TypeDrop {
		tmpl = "drop.gohtml"
	}

	data := struct {
		*model.Link
		Host    string
		Success string
	}{
		Link:    l,
		Host:    r.Host,
		Success: r.URL.Query().Get("success"),
	}

	if err := h.view.Render(w, tmpl, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
