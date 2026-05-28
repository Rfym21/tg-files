package web

import (
	"context"
	"io"

	"github.com/aahl/tgnas/metadata"
	"github.com/aahl/tgnas/store"
)

type ObjectStore interface {
	HeadBucket(ctx context.Context, name string) error
	PutObject(ctx context.Context, input store.PutObjectInput) (store.PutObjectResult, error)
	GetObject(ctx context.Context, input store.GetObjectInput) (io.ReadCloser, store.ObjectInfo, error)
	HeadObject(ctx context.Context, bucket, key string) (store.ObjectInfo, error)
	ListObjects(ctx context.Context, input store.ListObjectsInput) (store.ListObjectsResult, error)
	DeleteObject(ctx context.Context, bucket, key string) error
}

type MetadataStore interface {
	ListBuckets(ctx context.Context) ([]metadata.Bucket, error)
	GetBucket(ctx context.Context, name string) (metadata.Bucket, error)
	CreateDirectLink(ctx context.Context, link metadata.DirectLink) error
	GetDirectLinkByToken(ctx context.Context, token string) (metadata.DirectLink, error)
	ListDirectLinks(ctx context.Context, query metadata.ListLinksQuery) ([]metadata.DirectLink, error)
	RevokeDirectLink(ctx context.Context, token string) error
	IncrementDirectLinkClick(ctx context.Context, token string) error
}

type BucketInfo struct {
	Name       string `json:"name"`
	PublicRead bool   `json:"public_read"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiErrorResponse struct {
	Error apiError `json:"error"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	User      string `json:"user"`
	ExpiresAt int64  `json:"expires_at"`
}

type meResponse struct {
	User      string `json:"user"`
	ExpiresAt int64  `json:"expires_at"`
}

type objectDTO struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	ContentType  string `json:"content_type"`
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
}

type listFilesResponse struct {
	Objects     []objectDTO `json:"objects"`
	NextAfter   string      `json:"next_after,omitempty"`
	IsTruncated bool        `json:"is_truncated"`
}

type uploadResponse struct {
	Object objectDTO  `json:"object"`
	Link   *linkDTO   `json:"link,omitempty"`
	Bucket string     `json:"bucket"`
}

type linkDTO struct {
	Token      string `json:"token"`
	URL        string `json:"url"`
	Bucket     string `json:"bucket"`
	Key        string `json:"key"`
	Filename   string `json:"filename,omitempty"`
	CreatedAt  string `json:"created_at"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	ClickCount int64  `json:"click_count"`
	Revoked    bool   `json:"revoked"`
}

type createLinkRequest struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	TTLSeconds  int64  `json:"ttl_seconds"`
	Filename    string `json:"filename"`
}

type listLinksResponse struct {
	Links     []linkDTO `json:"links"`
	NextAfter string    `json:"next_after,omitempty"`
}
