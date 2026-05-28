package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aahl/tgnas/metadata"
	"github.com/aahl/tgnas/store"
)

type fakeStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	infos     map[string]store.ObjectInfo
	putErr    error
	headErr   error
	getErr    error
	deleteErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}, infos: map[string]store.ObjectInfo{}}
}

func key(bucket, k string) string { return bucket + "/" + k }

func (f *fakeStore) HeadBucket(ctx context.Context, name string) error { return nil }

func (f *fakeStore) PutObject(ctx context.Context, input store.PutObjectInput) (store.PutObjectResult, error) {
	if f.putErr != nil {
		return store.PutObjectResult{}, f.putErr
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return store.PutObjectResult{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key(input.Bucket, input.Key)] = body
	f.infos[key(input.Bucket, input.Key)] = store.ObjectInfo{
		Bucket: input.Bucket, Key: input.Key, Size: int64(len(body)),
		ContentType: input.ContentType, ETag: "etag-" + input.Key,
		LastModified: time.Unix(1_700_000_000, 0).UTC(),
	}
	return store.PutObjectResult{ETag: "etag-" + input.Key}, nil
}

func (f *fakeStore) GetObject(ctx context.Context, input store.GetObjectInput) (io.ReadCloser, store.ObjectInfo, error) {
	if f.getErr != nil {
		return nil, store.ObjectInfo{}, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	body, ok := f.objects[key(input.Bucket, input.Key)]
	if !ok {
		return nil, store.ObjectInfo{}, store.ErrNoSuchKey
	}
	info := f.infos[key(input.Bucket, input.Key)]
	if input.Range != nil {
		body = body[input.Range.Start : input.Range.End+1]
	}
	return io.NopCloser(bytes.NewReader(body)), info, nil
}

func (f *fakeStore) HeadObject(ctx context.Context, bucket, k string) (store.ObjectInfo, error) {
	if f.headErr != nil {
		return store.ObjectInfo{}, f.headErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	info, ok := f.infos[key(bucket, k)]
	if !ok {
		return store.ObjectInfo{}, store.ErrNoSuchKey
	}
	return info, nil
}

func (f *fakeStore) ListObjects(ctx context.Context, input store.ListObjectsInput) (store.ListObjectsResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var objs []store.ObjectInfo
	for _, info := range f.infos {
		if info.Bucket != input.Bucket {
			continue
		}
		if input.Prefix != "" && !strings.HasPrefix(info.Key, input.Prefix) {
			continue
		}
		objs = append(objs, info)
	}
	return store.ListObjectsResult{Objects: objs}, nil
}

func (f *fakeStore) DeleteObject(ctx context.Context, bucket, k string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.objects[key(bucket, k)]; !ok {
		return store.ErrNoSuchKey
	}
	delete(f.objects, key(bucket, k))
	delete(f.infos, key(bucket, k))
	return nil
}

type fakeMeta struct {
	mu     sync.Mutex
	links  map[string]metadata.DirectLink
	createErr error
}

func newFakeMeta() *fakeMeta {
	return &fakeMeta{links: map[string]metadata.DirectLink{}}
}

func (m *fakeMeta) ListBuckets(ctx context.Context) ([]metadata.Bucket, error) {
	return []metadata.Bucket{{Name: "demo", Enabled: true}}, nil
}

func (m *fakeMeta) GetBucket(ctx context.Context, name string) (metadata.Bucket, error) {
	return metadata.Bucket{Name: name, Enabled: true}, nil
}

func (m *fakeMeta) CreateDirectLink(ctx context.Context, link metadata.DirectLink) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links[link.Token] = link
	return nil
}

func (m *fakeMeta) GetDirectLinkByToken(ctx context.Context, token string) (metadata.DirectLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	link, ok := m.links[token]
	if !ok {
		return metadata.DirectLink{}, metadata.ErrNotFound
	}
	return link, nil
}

func (m *fakeMeta) ListDirectLinks(ctx context.Context, q metadata.ListLinksQuery) ([]metadata.DirectLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []metadata.DirectLink
	for _, link := range m.links {
		if q.Bucket != "" && link.Bucket != q.Bucket {
			continue
		}
		out = append(out, link)
	}
	return out, nil
}

func (m *fakeMeta) RevokeDirectLink(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	link, ok := m.links[token]
	if !ok {
		return metadata.ErrNotFound
	}
	link.Revoked = true
	m.links[token] = link
	return nil
}

func (m *fakeMeta) IncrementDirectLinkClick(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	link, ok := m.links[token]
	if !ok {
		return metadata.ErrNotFound
	}
	link.ClickCount++
	m.links[token] = link
	return nil
}

func newTestServer() (*Server, *fakeStore, *fakeMeta) {
	st := newFakeStore()
	mt := newFakeMeta()
	srv := NewServer(st, mt, Options{
		AdminUser:            "admin",
		AdminPassword:        "changeme",
		SessionSecret:        []byte("01234567890123456789012345678901"),
		SessionTTL:           time.Hour,
		DirectLinkDefaultTTL: time.Hour,
		UploadAutoLink:       true,
		BucketChatIDs:        map[string]string{"demo": "-100"},
		PublicReadBuckets:    map[string]bool{},
	})
	return srv, st, mt
}

func loginAndCookie(t *testing.T, srv *Server) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "changeme"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func TestLoginAndMe(t *testing.T) {
	srv, _, _ := newTestServer()
	cookie := loginAndCookie(t, srv)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}
	var resp meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.User != "admin" {
		t.Errorf("user = %q", resp.User)
	}
}

func TestLoginBadCredentials(t *testing.T) {
	srv, _, _ := newTestServer()
	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "wrong"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRequireAuthRejectsMissingCookie(t *testing.T) {
	srv, _, _ := newTestServer()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/buckets", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUploadCreatesAutoLink(t *testing.T) {
	srv, st, mt := newTestServer()
	cookie := loginAndCookie(t, srv)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/files?bucket=demo&key=hello.txt", strings.NewReader("hello"))
	r.Header.Set("Content-Type", "text/plain")
	r.ContentLength = 5
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Link == nil {
		t.Fatal("expected auto-link, got nil")
	}
	if resp.Link.URL == "" || !strings.Contains(resp.Link.URL, "/d/"+resp.Link.Token) {
		t.Errorf("link URL = %q", resp.Link.URL)
	}
	if _, ok := st.objects[key("demo", "hello.txt")]; !ok {
		t.Errorf("object not stored")
	}
	if _, ok := mt.links[resp.Link.Token]; !ok {
		t.Errorf("link not persisted")
	}
}

func TestDirectLinkServesContent(t *testing.T) {
	srv, _, _ := newTestServer()
	cookie := loginAndCookie(t, srv)

	// upload
	up := httptest.NewRequest(http.MethodPost, "/api/v1/files?bucket=demo&key=hello.txt", strings.NewReader("hello"))
	up.Header.Set("Content-Type", "text/plain")
	up.ContentLength = 5
	up.AddCookie(cookie)
	upRec := httptest.NewRecorder()
	srv.ServeHTTP(upRec, up)
	if upRec.Code != http.StatusCreated {
		t.Fatalf("upload failed: %d", upRec.Code)
	}
	var resp uploadResponse
	_ = json.Unmarshal(upRec.Body.Bytes(), &resp)

	// fetch via /d/<token>
	dlReq := httptest.NewRequest(http.MethodGet, "/d/"+resp.Link.Token, nil)
	dlRec := httptest.NewRecorder()
	srv.ServeHTTP(dlRec, dlReq)
	if dlRec.Code != http.StatusOK {
		t.Fatalf("direct link status = %d body=%s", dlRec.Code, dlRec.Body.String())
	}
	if dlRec.Body.String() != "hello" {
		t.Errorf("body = %q", dlRec.Body.String())
	}
	if dlRec.Header().Get("Accept-Ranges") != "bytes" {
		t.Errorf("missing Accept-Ranges")
	}

	// expired link
	dlReqMissing := httptest.NewRequest(http.MethodGet, "/d/nonexistent", nil)
	dlRecMissing := httptest.NewRecorder()
	srv.ServeHTTP(dlRecMissing, dlReqMissing)
	if dlRecMissing.Code != http.StatusNotFound {
		t.Errorf("missing token status = %d", dlRecMissing.Code)
	}
}

func TestDirectLinkRange(t *testing.T) {
	srv, _, _ := newTestServer()
	cookie := loginAndCookie(t, srv)
	up := httptest.NewRequest(http.MethodPost, "/api/v1/files?bucket=demo&key=hello.txt", strings.NewReader("hello"))
	up.Header.Set("Content-Type", "text/plain")
	up.ContentLength = 5
	up.AddCookie(cookie)
	upRec := httptest.NewRecorder()
	srv.ServeHTTP(upRec, up)
	var resp uploadResponse
	_ = json.Unmarshal(upRec.Body.Bytes(), &resp)

	r := httptest.NewRequest(http.MethodGet, "/d/"+resp.Link.Token, nil)
	r.Header.Set("Range", "bytes=0-0")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "h" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestRevokeLinkDeniesAccess(t *testing.T) {
	srv, _, mt := newTestServer()
	cookie := loginAndCookie(t, srv)
	up := httptest.NewRequest(http.MethodPost, "/api/v1/files?bucket=demo&key=x", strings.NewReader("x"))
	up.Header.Set("Content-Type", "text/plain")
	up.ContentLength = 1
	up.AddCookie(cookie)
	upRec := httptest.NewRecorder()
	srv.ServeHTTP(upRec, up)
	var resp uploadResponse
	_ = json.Unmarshal(upRec.Body.Bytes(), &resp)
	if err := mt.RevokeDirectLink(context.Background(), resp.Link.Token); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/d/"+resp.Link.Token, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("revoked status = %d", rec.Code)
	}
}

func TestExpiredLinkRejected(t *testing.T) {
	srv, _, mt := newTestServer()
	cookie := loginAndCookie(t, srv)
	up := httptest.NewRequest(http.MethodPost, "/api/v1/files?bucket=demo&key=expired", strings.NewReader("hi"))
	up.Header.Set("Content-Type", "text/plain")
	up.ContentLength = 2
	up.AddCookie(cookie)
	upRec := httptest.NewRecorder()
	srv.ServeHTTP(upRec, up)
	var resp uploadResponse
	_ = json.Unmarshal(upRec.Body.Bytes(), &resp)

	mt.mu.Lock()
	link := mt.links[resp.Link.Token]
	link.ExpiresAt = time.Now().Add(-time.Hour)
	mt.links[resp.Link.Token] = link
	mt.mu.Unlock()

	r := httptest.NewRequest(http.MethodGet, "/d/"+resp.Link.Token, nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestOwnsPath(t *testing.T) {
	srv, _, _ := newTestServer()
	cases := map[string]bool{
		"/api/v1/anything":  true,
		"/d/abc":            true,
		"/assets/x.js":      true,
		"/login":            true,
		"/files":            true,
		"/links":            true,
		"/favicon.ico":      true,
		"/some-bucket":      false,
		"/dav/foo":          false,
		"/healthz":          false,
	}
	for path, want := range cases {
		if got := srv.OwnsPath(path); got != want {
			t.Errorf("OwnsPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsBrowserAnonymous(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/foo", nil)
	r.Header.Set("Accept", "text/html,application/xhtml+xml")
	if !IsBrowserAnonymous(r) {
		t.Fatal("expected browser anonymous")
	}
	r.Header.Set("X-Amz-Date", "20260101T000000Z")
	if IsBrowserAnonymous(r) {
		t.Fatal("expected false with x-amz header")
	}
}

func TestUploadRejectsUnknownBucket(t *testing.T) {
	srv, _, _ := newTestServer()
	cookie := loginAndCookie(t, srv)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/files?bucket=missing&key=x", strings.NewReader("x"))
	r.Header.Set("Content-Type", "text/plain")
	r.ContentLength = 1
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUploadHeadObjectErrorPropagates(t *testing.T) {
	srv, st, _ := newTestServer()
	cookie := loginAndCookie(t, srv)
	st.headErr = errors.New("head failed")
	r := httptest.NewRequest(http.MethodPost, "/api/v1/files?bucket=demo&key=x", strings.NewReader("x"))
	r.Header.Set("Content-Type", "text/plain")
	r.ContentLength = 1
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
