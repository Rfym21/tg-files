package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aahl/tgnas/metadata"
	"github.com/aahl/tgnas/store"
)

func (s *Server) handleDirectLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	link, err := s.meta.GetDirectLinkByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, metadata.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.Printf("debug event=direct_link_lookup_error token=%q error=%v", token, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if link.Revoked {
		http.NotFound(w, r)
		return
	}
	if !link.ExpiresAt.IsZero() && link.ExpiresAt.Before(time.Now()) {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodHead {
		info, err := s.store.HeadObject(r.Context(), link.Bucket, link.Key)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		s.writeObjectHeaders(w, r, link, info, nil)
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.WriteHeader(http.StatusOK)
		return
	}

	var requestedRange *store.ByteRange
	status := http.StatusOK
	if header := r.Header.Get("Range"); header != "" {
		info, err := s.store.HeadObject(r.Context(), link.Bucket, link.Key)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		parsed, err := store.ParseRange(header, info.Size)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		requestedRange = &parsed
		status = http.StatusPartialContent
	}
	reader, info, err := s.store.GetObject(r.Context(), store.GetObjectInput{
		Bucket: link.Bucket,
		Key:    link.Key,
		Range:  requestedRange,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	defer reader.Close()

	s.writeObjectHeaders(w, r, link, info, requestedRange)
	length := info.Size
	if requestedRange != nil {
		length = requestedRange.Length()
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", requestedRange.Start, requestedRange.End, info.Size))
	}
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(status)
	_, _ = io.Copy(w, reader)

	go func(tok string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.meta.IncrementDirectLinkClick(ctx, tok); err != nil {
			s.logger.Printf("debug event=direct_link_click_increment_failed token=%q error=%v", tok, err)
		}
	}(link.Token)
}

func (s *Server) writeObjectHeaders(w http.ResponseWriter, r *http.Request, link metadata.DirectLink, info store.ObjectInfo, _ *store.ByteRange) {
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", info.ETag))
	if info.ContentType != "" {
		w.Header().Set("Content-Type", info.ContentType)
	}
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
	filename := link.Filename
	if r.URL.Query().Get("download") == "1" && filename == "" {
		filename = baseName(link.Key)
	}
	if filename != "" {
		w.Header().Set("Content-Disposition", contentDisposition(filename, r.URL.Query().Get("download") == "1"))
	}
}

func contentDisposition(filename string, forceAttachment bool) string {
	sanitized := strings.NewReplacer("\"", "", "\\", "", "\r", "", "\n", "").Replace(filename)
	disposition := "inline"
	if forceAttachment {
		disposition = "attachment"
	}
	return fmt.Sprintf(`%s; filename="%s"`, disposition, sanitized)
}
