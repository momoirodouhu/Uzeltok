package handler

import (
	"crypto/subtle"
	"net/http"
	"time"

	"uzeltok/internal/model"
	"uzeltok/internal/store"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

// --- Auth middleware ---

// adminAuth は Basic 認証ミドルウェアです（パスワードのみ検証、ユーザー名は任意）。
// pass が空の場合は全てのリクエストを 403 Forbidden で拒否します。
func (h *Handler) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	pass := h.adminPass
	return func(w http.ResponseWriter, r *http.Request) {
		if pass == "" {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("WWW-Authenticate", `Basic realm="Uzeltok Admin"`)
			w.WriteHeader(http.StatusUnauthorized)
			if err := h.view.Render(w, "401.gohtml", nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		_, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			time.Sleep(time.Second) // 気持ち程度のブルートフォース攻撃対策
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("WWW-Authenticate", `Basic realm="Uzeltok Admin"`)
			w.WriteHeader(http.StatusUnauthorized)
			if err := h.view.Render(w, "401.gohtml", nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		next(w, r)
	}
}

// --- Admin handlers ---

// handleAdmin は全リンクの一覧を表示する管理画面を返します。
func (h *Handler) handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
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

	data := struct {
		Links []*model.Link
		Host  string
	}{
		Links: links,
		Host:  r.Host,
	}

	if err := h.view.Render(w, "admin.gohtml", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAdminDetail はリンク詳細画面を返します。
func (h *Handler) handleAdminDetail(w http.ResponseWriter, r *http.Request) {
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

	data := struct {
		*model.Link
		Host string
	}{
		Link: l,
		Host: r.Host,
	}

	if err := h.view.Render(w, "admin_link.gohtml", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAdminUpload は管理者によるファイルアップロードを処理します。
func (h *Handler) handleAdminUpload(w http.ResponseWriter, r *http.Request) {
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

	http.Redirect(w, r, "/admin/links/"+l.ID+"?success=upload", http.StatusSeeOther)
}

// handleAdminDeleteFile は管理者によるファイル削除を処理します。
func (h *Handler) handleAdminDeleteFile(w http.ResponseWriter, r *http.Request) {
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

	filename := r.PathValue("filename")
	if filename == "" {
		h.notFound(w, r)
		return
	}

	if err := h.store.DeleteFile(l.ID, filename); err != nil {
		if err == store.ErrNotFound {
			h.notFound(w, r)
			return
		}
		http.Error(w, "failed to delete file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/links/"+l.ID, http.StatusSeeOther)
}

// handleAdminDownloadFile は管理者によるファイルダウンロードを処理します。
func (h *Handler) handleAdminDownloadFile(w http.ResponseWriter, r *http.Request) {
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

	filename := r.PathValue("filename")
	if filename == "" {
		h.notFound(w, r)
		return
	}

	h.serveFile(w, r, l, filename)
}

// handleAdminCreateLink は新しいリンクを作成します。
func (h *Handler) handleAdminCreateLink(w http.ResponseWriter, r *http.Request) {
	linkType := r.FormValue("type")
	expiresIn := r.FormValue("expires_in")

	id, err := gonanoid.Generate("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", 14)
	if err != nil {
		http.Error(w, "failed to generate random ID", http.StatusInternalServerError)
		return
	}

	l := &model.Link{
		ID: id,
		Metadata: model.Metadata{
			CreatedAt: time.Now(),
		},
	}

	if linkType == "share" {
		l.Metadata.Type = model.TypeShare
	} else {
		l.Metadata.Type = model.TypeDrop
	}

	if expiresIn != "" && expiresIn != "never" {
		d, err := time.ParseDuration(expiresIn)
		if err == nil {
			l.Metadata.ExpiresAt = time.Now().Add(d)
		}
	}

	if err := h.store.CreateLink(l); err != nil {
		http.Error(w, "Failed to create link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/links/"+l.ID, http.StatusSeeOther)
}

// handleAdminDeleteLink はリンクを物理的または論理的に削除します。
func (h *Handler) handleAdminDeleteLink(w http.ResponseWriter, r *http.Request) {
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

	if err := h.store.DeleteLink(l.ID); err != nil {
		if err == store.ErrNotFound {
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
