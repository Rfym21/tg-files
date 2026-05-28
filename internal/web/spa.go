package web

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

const spaUnbuiltMessage = "web UI not built; run `npm run build` in web/"

type spaHandler struct {
	root        fs.FS
	hasIndex    bool
	hasAssets   bool
	indexBytes  []byte
	indexModTime time.Time
}

func newSPAHandler(root fs.FS) http.Handler {
	if root == nil {
		return &spaHandler{}
	}
	sub, err := fs.Sub(root, "dist")
	if err != nil {
		return &spaHandler{}
	}
	h := &spaHandler{root: sub}
	if data, err := fs.ReadFile(sub, "index.html"); err == nil {
		h.indexBytes = data
		h.hasIndex = true
		if info, err := fs.Stat(sub, "index.html"); err == nil {
			h.indexModTime = info.ModTime()
		}
	}
	if entries, err := fs.ReadDir(sub, "assets"); err == nil && len(entries) > 0 {
		h.hasAssets = true
	}
	return h
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.hasIndex {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, spaUnbuiltMessage)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		h.serveIndex(w, r)
		return
	}

	file, err := h.root.Open(path)
	if err == nil {
		stat, statErr := fs.Stat(h.root, path)
		if statErr == nil && !stat.IsDir() {
			defer file.Close()
			if strings.HasPrefix(path, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeContent(w, r, stat.Name(), stat.ModTime(), readSeekerFromFile(file))
			return
		}
		_ = file.Close()
	} else if !errors.Is(err, fs.ErrNotExist) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "spa read error")
		return
	}

	h.serveIndex(w, r)
}

func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", h.indexModTime, newBytesReadSeeker(h.indexBytes))
}

type bytesReadSeeker struct {
	data []byte
	pos  int64
}

func newBytesReadSeeker(data []byte) *bytesReadSeeker {
	return &bytesReadSeeker{data: data}
}

func (b *bytesReadSeeker) Read(p []byte) (int, error) {
	if b.pos >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += int64(n)
	return n, nil
}

func (b *bytesReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = b.pos + offset
	case io.SeekEnd:
		abs = int64(len(b.data)) + offset
	default:
		return 0, errors.New("bytesReadSeeker.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("bytesReadSeeker.Seek: negative position")
	}
	b.pos = abs
	return abs, nil
}

func readSeekerFromFile(file fs.File) io.ReadSeeker {
	if rs, ok := file.(io.ReadSeeker); ok {
		return rs
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return newBytesReadSeeker(nil)
	}
	return newBytesReadSeeker(data)
}
