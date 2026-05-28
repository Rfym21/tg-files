package web

import (
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

const sessionCookieName = "tgnas_session"

type Options struct {
	AdminUser            string
	AdminPassword        string
	SessionSecret        []byte
	SessionTTL           time.Duration
	DirectLinkDefaultTTL time.Duration
	UploadAutoLink       bool
	AllowedBuckets       []string
	PublicReadBuckets    map[string]bool
	BucketChatIDs        map[string]string
	PublicBaseURL        string
	Logger               *log.Logger
}

type Server struct {
	opts    Options
	store   ObjectStore
	meta    MetadataStore
	logger  *log.Logger
	allowed map[string]struct{}
	mux     *http.ServeMux
	spa     http.Handler
}

func NewServer(store ObjectStore, meta MetadataStore, opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = log.New(noopWriter{}, "", 0)
	}
	allowed := map[string]struct{}{}
	for _, name := range opts.AllowedBuckets {
		allowed[name] = struct{}{}
	}
	s := &Server{
		opts:    opts,
		store:   store,
		meta:    meta,
		logger:  logger,
		allowed: allowed,
	}
	s.spa = newSPAHandler(distFS)
	s.mux = http.NewServeMux()
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.requireAuth(s.handleLogout))
	s.mux.HandleFunc("GET /api/v1/auth/me", s.requireAuth(s.handleMe))

	s.mux.HandleFunc("GET /api/v1/buckets", s.requireAuth(s.handleListBuckets))

	s.mux.HandleFunc("GET /api/v1/files", s.requireAuth(s.handleListFiles))
	s.mux.HandleFunc("POST /api/v1/files", s.requireAuth(s.handleUploadFile))
	s.mux.HandleFunc("DELETE /api/v1/files", s.requireAuth(s.handleDeleteFile))
	s.mux.HandleFunc("GET /api/v1/files/head", s.requireAuth(s.handleHeadFile))

	s.mux.HandleFunc("POST /api/v1/links", s.requireAuth(s.handleCreateLink))
	s.mux.HandleFunc("GET /api/v1/links", s.requireAuth(s.handleListLinks))
	s.mux.HandleFunc("DELETE /api/v1/links/{token}", s.requireAuth(s.handleRevokeLink))

	s.mux.HandleFunc("GET /d/{token}", s.handleDirectLink)
	s.mux.HandleFunc("HEAD /d/{token}", s.handleDirectLink)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.isAPIPath(r.URL.Path) || isDirectLinkPath(r.URL.Path) {
		s.recoverPanic(s.mux).ServeHTTP(w, r)
		return
	}
	s.spa.ServeHTTP(w, r)
}

// OwnsPath returns true when the request path falls inside a prefix
// reserved for the web UI. Used by the outer combined handler.
func (s *Server) OwnsPath(path string) bool {
	if s.isAPIPath(path) || isDirectLinkPath(path) {
		return true
	}
	if path == "/favicon.ico" || path == "/manifest.webmanifest" {
		return true
	}
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	switch path {
	case "/login", "/files", "/links":
		return true
	}
	return false
}

func (s *Server) isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

func isDirectLinkPath(path string) bool {
	return strings.HasPrefix(path, "/d/")
}

func (s *Server) bucketAllowed(name string) bool {
	if len(s.allowed) == 0 {
		_, ok := s.opts.BucketChatIDs[name]
		return ok
	}
	_, ok := s.allowed[name]
	return ok
}

func (s *Server) allowedBucketList() []BucketInfo {
	out := make([]BucketInfo, 0)
	for name, _ := range s.opts.BucketChatIDs {
		if len(s.allowed) > 0 {
			if _, ok := s.allowed[name]; !ok {
				continue
			}
		}
		out = append(out, BucketInfo{Name: name, PublicRead: s.opts.PublicReadBuckets[name]})
	}
	return out
}

// IsBrowserAnonymous reports whether the request looks like a browser GET
// without any S3 SigV4 markers. Used as the SPA fallback predicate.
func IsBrowserAnonymous(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if r.Header.Get("Authorization") != "" {
		return false
	}
	for name := range r.Header {
		if strings.HasPrefix(strings.ToLower(name), "x-amz-") {
			return false
		}
	}
	for key := range r.URL.Query() {
		if strings.HasPrefix(strings.ToLower(key), "x-amz-") {
			return false
		}
	}
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	parts := strings.Split(strings.ToLower(accept), ",")
	for _, part := range parts {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mediaType == "text/html" {
			return true
		}
		if mediaType == "application/xml" || mediaType == "text/xml" || mediaType == "*/*" {
			return false
		}
	}
	return false
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

// distFS is bound in spa_dist.go via //go:embed.
var distFS fs.FS
