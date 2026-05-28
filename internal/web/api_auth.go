package web

import (
	"crypto/subtle"
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "application/json required")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.opts.AdminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.opts.AdminPassword)) == 1
	if !userOK || !passOK {
		jitter := time.Duration(50+rand.Intn(100)) * time.Millisecond
		time.Sleep(jitter)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "username or password is incorrect")
		return
	}
	now := time.Now()
	token, err := signSessionToken(s.opts.SessionSecret, s.opts.AdminUser, now, s.opts.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to sign session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.opts.SessionTTL.Seconds()),
	})
	expires := now.Add(s.opts.SessionTTL)
	writeJSON(w, http.StatusOK, loginResponse{User: s.opts.AdminUser, ExpiresAt: expires.Unix()})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "no session")
		return
	}
	claims, err := parseSessionToken(s.opts.SessionSecret, cookie.Value, time.Now())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid session")
		return
	}
	writeJSON(w, http.StatusOK, meResponse{User: claims.Sub, ExpiresAt: claims.Exp})
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	if strings.EqualFold(r.URL.Scheme, "https") {
		return true
	}
	return false
}
