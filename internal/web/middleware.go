package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type ctxKey int

const ctxKeyUser ctxKey = iota

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiErrorResponse{Error: apiError{Code: code, Message: message}})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "session required")
			return
		}
		claims, err := parseSessionToken(s.opts.SessionSecret, cookie.Value, time.Now())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid session")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUser, claims.Sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func userFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUser).(string); ok {
		return v
	}
	return ""
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Printf("debug event=web_panic path=%q recover=%v", r.URL.Path, rec)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
