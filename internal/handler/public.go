package handler

import (
	"net/http"
	"strings"

	"uzeltok/internal/model"
	"uzeltok/internal/store"
)

// handleLink は /{uuid} と /{uuid}/{filename} を処理する公開ハンドラです。
func (h *Handler) handleLink(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		if err := h.view.Render(w, "index.gohtml", nil); err != nil {
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

	// リンクを1回だけ取得
	l, err := h.fetchLink(uuid)
	if err != nil {
		if err == store.ErrNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// /{uuid} — リンク情報ページ
	if len(parts) == 1 || parts[1] == "" {
		h.renderLinkPage(w, l)
		return
	}

	// /{uuid}/{filename} — ファイルダウンロード
	h.serveFile(w, r, l, parts[1])
}

// renderLinkPage はリンクの種類に応じたテンプレートを描画します。
func (h *Handler) renderLinkPage(w http.ResponseWriter, l *model.Link) {
	tmpl := "share.gohtml"
	if l.Metadata.Type == model.TypeDrop {
		tmpl = "drop.gohtml"
	}
	if err := h.view.Render(w, tmpl, l); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
