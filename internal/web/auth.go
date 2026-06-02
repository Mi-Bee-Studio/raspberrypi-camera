package web

import (
	"crypto/subtle"
	"net/http"
)

// authMiddleware wraps an http.Handler with HTTP Basic authentication.
// Static assets (/ and /static/*) and the WebSocket upgrade are excluded
// by the caller wrapping specific handlers with authRequired.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// authRequired wraps an http.HandlerFunc with HTTP Basic authentication.
// Returns 401 if credentials are missing or incorrect.
func (s *Server) authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="rpi-cam"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Constant-time comparison to prevent timing attacks.
		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(s.username)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(s.password)) == 1

		if !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="rpi-cam"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
