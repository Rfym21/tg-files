package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aahl/tgnas/metadata"
)

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "application/json required")
		return
	}
	var req createLinkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if req.Bucket == "" || req.Key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key are required")
		return
	}
	if !s.bucketAllowed(req.Bucket) {
		writeError(w, http.StatusForbidden, "forbidden", "bucket not allowed")
		return
	}
	ttl := s.opts.DirectLinkDefaultTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	} else if req.TTLSeconds < 0 {
		ttl = 0
	}
	filename := req.Filename
	if filename == "" {
		filename = baseName(req.Key)
	}
	link, err := s.createLink(r.Context(), req.Bucket, req.Key, filename, ttl, userFromContext(r.Context()))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.linkToDTO(r, link))
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	if bucket != "" && !s.bucketAllowed(bucket) {
		writeError(w, http.StatusForbidden, "forbidden", "bucket not allowed")
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	links, err := s.meta.ListDirectLinks(r.Context(), metadata.ListLinksQuery{
		Bucket:   bucket,
		AfterTok: r.URL.Query().Get("after"),
		Limit:    limit,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := make([]linkDTO, 0, len(links))
	for _, link := range links {
		out = append(out, s.linkToDTO(r, link))
	}
	resp := listLinksResponse{Links: out}
	if len(out) > 0 && len(links) == limit {
		resp.NextAfter = links[len(links)-1].Token
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRevokeLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "token is required")
		return
	}
	if err := s.meta.RevokeDirectLink(r.Context(), token); err != nil {
		if errors.Is(err, metadata.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) linkToDTO(r *http.Request, link metadata.DirectLink) linkDTO {
	dto := linkDTO{
		Token:      link.Token,
		Bucket:     link.Bucket,
		Key:        link.Key,
		Filename:   link.Filename,
		CreatedAt:  link.CreatedAt.UTC().Format(time.RFC3339),
		ClickCount: link.ClickCount,
		Revoked:    link.Revoked,
		URL:        s.buildLinkURL(r, link.Token),
	}
	if !link.ExpiresAt.IsZero() {
		dto.ExpiresAt = link.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return dto
}

func (s *Server) buildLinkURL(r *http.Request, token string) string {
	base := strings.TrimRight(s.opts.PublicBaseURL, "/")
	if base != "" {
		return base + "/d/" + token
	}
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host + "/d/" + token
}

func baseName(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}
