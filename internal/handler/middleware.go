package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

const csrfCookieName = "uzeltok_csrf"
const csrfFormField = "csrf_token"

// newCSRFToken は 64 文字の暗号論的乱数 hex 文字列を返します。
func newCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ensureCSRFCookie は CSRF トークン Cookie が存在しない場合に新規生成してセットし、
// トークン値を返します。管理者 GET ハンドラからテンプレートデータへ渡してください。
func ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil && len(c.Value) == 64 {
		return c.Value
	}
	token, err := newCSRFToken()
	if err != nil {
		return "" // crypto/rand 失敗時は空文字列; POST 側で弾かれる
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return token
}

// csrfProtect は csrf_token フォームフィールドと Cookie を比較する
// Double Submit Cookie ミドルウェアです。変更系 POST ルートにのみ適用します。
func csrfProtect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || len(cookie.Value) != 64 {
			http.Error(w, "CSRF validation failed", http.StatusForbidden)
			return
		}
		formToken := r.FormValue(csrfFormField)
		if subtle.ConstantTimeCompare([]byte(formToken), []byte(cookie.Value)) != 1 {
			http.Error(w, "CSRF validation failed", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// securityHeaders は全レスポンスに共通のセキュリティヘッダーを設定するミドルウェアです。
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
		w.Header().Set("Vary", "Accept-Encoding")
		next.ServeHTTP(w, r)
	})
}
