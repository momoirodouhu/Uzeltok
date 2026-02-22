package handler

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"uzeltok/internal/model"
	"uzeltok/internal/store"
)

// --- Auth middleware ---

// adminAuth は Basic 認証ミドルウェアです（パスワードのみ検証、ユーザー名は任意）。
// pass が空の場合は全てのリクエストを 403 Forbidden で拒否します。
func adminAuth(next http.HandlerFunc, pass string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if pass == "" {
			http.Error(w, "admin access is disabled", http.StatusForbidden)
			return
		}
		_, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Uzeltok Admin"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
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

	if err := h.view.Render(w, "admin.gohtml", links); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAdminSub は /admin/{id}, /admin/{id}/upload, /admin/{id}/delete,
// /admin/{id}/{filename} を処理します。
func (h *Handler) handleAdminSub(w http.ResponseWriter, r *http.Request) {
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

	// /admin/{id} — リンク詳細
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}

	switch sub {
	case "":
		h.handleAdminDetail(w, r, l)
	case "upload":
		h.handleAdminUpload(w, r, l)
	case "delete":
		h.handleAdminDelete(w, r, l)
	default:
		// /admin/{id}/{filename} — ファイルダウンロード
		h.serveFile(w, r, l, sub)
	}
}

// handleAdminDetail はリンク詳細画面を返します。
func (h *Handler) handleAdminDetail(w http.ResponseWriter, r *http.Request, l *model.Link) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.view.Render(w, "admin_link.gohtml", l); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
