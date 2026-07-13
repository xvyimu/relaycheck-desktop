package core

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

// sessionTokenCookie is the HttpOnly cookie carrying the opt-in local session
// token. When token enforcement is enabled, every /api/* handler (except
// /api/health) requires this cookie to match the process token. The SPA never
// reads it directly: the browser attaches it automatically on same-origin
// fetches (credentials: "same-origin"), so no frontend change is required.
const sessionTokenCookie = "rc_session"

// NewSessionToken returns a fresh 256-bit random hex token, or "" if the
// system RNG fails (caller should treat "" as "do not enable enforcement").
func NewSessionToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

// SetLocalToken enables session-token enforcement with the given token. An
// empty token disables enforcement (default). Safe to call once at startup.
func (a *App) SetLocalToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.localToken = token
}

// tokenEnforced reports whether a non-empty token is configured.
func (a *App) tokenEnforced() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.localToken != ""
}

// localTokenValue returns the configured token (may be empty).
func (a *App) localTokenValue() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.localToken
}

// validateSessionToken checks the request's session cookie against the
// configured token using a constant-time comparison. It returns true when
// enforcement is disabled (no token configured).
func (a *App) validateSessionToken(r *http.Request) bool {
	expected := a.localTokenValue()
	if expected == "" {
		return true
	}
	cookie, err := r.Cookie(sessionTokenCookie)
	if err != nil {
		return false
	}
	got := strings.TrimSpace(cookie.Value)
	if len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// SetSessionCookieIfEnabled writes the session-token cookie on the response
// when enforcement is enabled. Called when serving index.html so the browser
// picks up the token on first load. No-op when enforcement is disabled.
func (a *App) SetSessionCookieIfEnabled(w http.ResponseWriter) {
	token := a.localTokenValue()
	if token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionTokenCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
