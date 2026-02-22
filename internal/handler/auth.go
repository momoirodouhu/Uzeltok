package handler

import (
	"crypto/subtle"
	"net/http"
)

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
