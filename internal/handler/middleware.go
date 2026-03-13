package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
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

func (h *Handler) signCSRFNonce(nonce string) string {
	mac := hmac.New(sha256.New, h.csrfSecret[:])
	_, _ = mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) isValidCSRFToken(token string) bool {
	nonce, sig, ok := strings.Cut(token, ".")
	if !ok || len(nonce) != 64 || len(sig) != 64 {
		return false
	}
	want := h.signCSRFNonce(nonce)
	return subtle.ConstantTimeCompare([]byte(sig), []byte(want)) == 1
}

// ensureCSRFCookie は CSRF トークン Cookie が存在しない場合に新規生成してセットし、
// トークン値を返します。管理者 GET ハンドラからテンプレートデータへ渡してください。
func (h *Handler) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil && h.isValidCSRFToken(c.Value) {
		return c.Value
	}
	nonce, err := newCSRFToken()
	if err != nil {
		return "" // crypto/rand 失敗時は空文字列; POST 側で弾かれる
	}
	token := nonce + "." + h.signCSRFNonce(nonce)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	return token
}

// csrfProtect は csrf_token フォームフィールドと Cookie を比較し、
// さらにサーバー署名を検証するミドルウェアです。変更系 POST ルートにのみ適用します。
func (h *Handler) csrfProtect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || !h.isValidCSRFToken(cookie.Value) {
			http.Error(w, "CSRF validation failed", http.StatusForbidden)
			return
		}
		formToken := r.FormValue(csrfFormField)
		if !h.isValidCSRFToken(formToken) {
			http.Error(w, "CSRF validation failed", http.StatusForbidden)
			return
		}
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
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		w.Header().Set("Vary", "Accept-Encoding")
		next.ServeHTTP(w, r)
	})
}
