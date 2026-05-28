package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aahl/tgnas/metadata"
	"github.com/aahl/tgnas/store"
)

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	if bucket == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket is required")
		return
	}
	if !s.bucketAllowed(bucket) {
		writeError(w, http.StatusForbidden, "forbidden", "bucket not allowed")
		return
	}
	prefix := r.URL.Query().Get("prefix")
	after := r.URL.Query().Get("after")
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	result, err := s.store.ListObjects(r.Context(), store.ListObjectsInput{
		Bucket:   bucket,
		Prefix:   prefix,
		AfterKey: after,
		Limit:    limit,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	objects := make([]objectDTO, 0, len(result.Objects))
	for _, obj := range result.Objects {
		objects = append(objects, objectDTO{
			Key:          obj.Key,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
			ETag:         obj.ETag,
			LastModified: obj.LastModified.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, listFilesResponse{
		Objects:     objects,
		NextAfter:   result.NextContinuationAfter,
		IsTruncated: result.IsTruncated,
	})
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	key := r.URL.Query().Get("key")
	if bucket == "" || key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key are required")
		return
	}
	if !s.bucketAllowed(bucket) {
		writeError(w, http.StatusForbidden, "forbidden", "bucket not allowed")
		return
	}
	if r.ContentLength <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "Content-Length is required")
		return
	}
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	result, err := s.store.PutObject(r.Context(), store.PutObjectInput{
		Bucket:      bucket,
		Key:         key,
		ContentType: contentType,
		Size:        r.ContentLength,
		Body:        r.Body,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	info, err := s.store.HeadObject(r.Context(), bucket, key)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	resp := uploadResponse{
		Bucket: bucket,
		Object: objectDTO{
			Key:          key,
			Size:         info.Size,
			ContentType:  info.ContentType,
			ETag:         result.ETag,
			LastModified: info.LastModified.UTC().Format(time.RFC3339),
		},
	}

	autoLink := s.opts.UploadAutoLink
	if raw := r.URL.Query().Get("create_link"); raw != "" {
		autoLink = raw == "1" || strings.EqualFold(raw, "true")
	}
	if autoLink {
		ttl := s.opts.DirectLinkDefaultTTL
		if raw := r.URL.Query().Get("ttl_seconds"); raw != "" {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
				ttl = time.Duration(parsed) * time.Second
			}
		}
		filename := path.Base(key)
		link, err := s.createLink(r.Context(), bucket, key, filename, ttl, userFromContext(r.Context()))
		if err != nil {
			s.logger.Printf("debug event=upload_auto_link_failed bucket=%q key=%q error=%v", bucket, key, err)
		} else {
			dto := s.linkToDTO(r, link)
			resp.Link = &dto
		}
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	key := r.URL.Query().Get("key")
	if bucket == "" || key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key are required")
		return
	}
	if !s.bucketAllowed(bucket) {
		writeError(w, http.StatusForbidden, "forbidden", "bucket not allowed")
		return
	}
	if err := s.store.DeleteObject(r.Context(), bucket, key); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHeadFile(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	key := r.URL.Query().Get("key")
	if bucket == "" || key == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket and key are required")
		return
	}
	if !s.bucketAllowed(bucket) {
		writeError(w, http.StatusForbidden, "forbidden", "bucket not allowed")
		return
	}
	info, err := s.store.HeadObject(r.Context(), bucket, key)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, objectDTO{
		Key:          info.Key,
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified.UTC().Format(time.RFC3339),
	})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNoSuchBucket):
		writeError(w, http.StatusNotFound, "no_such_bucket", err.Error())
	case errors.Is(err, store.ErrNoSuchKey):
		writeError(w, http.StatusNotFound, "no_such_key", err.Error())
	case errors.Is(err, store.ErrEntityTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "entity_too_large", err.Error())
	case errors.Is(err, store.ErrMissingContentLength):
		writeError(w, http.StatusLengthRequired, "missing_content_length", err.Error())
	case errors.Is(err, store.ErrInvalidRange):
		writeError(w, http.StatusRequestedRangeNotSatisfiable, "invalid_range", err.Error())
	case errors.Is(err, store.ErrInvalidArgument):
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, metadata.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func generateLinkToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Server) createLink(ctx context.Context, bucket, key, filename string, ttl time.Duration, createdBy string) (metadata.DirectLink, error) {
	if _, err := s.store.HeadObject(ctx, bucket, key); err != nil {
		return metadata.DirectLink{}, err
	}
	token, err := generateLinkToken()
	if err != nil {
		return metadata.DirectLink{}, err
	}
	now := time.Now()
	link := metadata.DirectLink{
		Token:     token,
		Bucket:    bucket,
		Key:       key,
		Filename:  filename,
		CreatedAt: now,
		CreatedBy: createdBy,
	}
	if ttl > 0 {
		link.ExpiresAt = now.Add(ttl)
	}
	if err := s.meta.CreateDirectLink(ctx, link); err != nil {
		return metadata.DirectLink{}, err
	}
	return link, nil
}
