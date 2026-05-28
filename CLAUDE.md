# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`tgnas` is a Go service that exposes Telegram as object storage via S3 (SigV4) and WebDAV. Object payload lives in Telegram messages; all object/chunk metadata lives in a local SQLite database. SQLite is the source of truth for listings, ETags, and chunk layout — Telegram is treated as opaque blob storage referenced by `file_id` / `message_id`.

## Common commands

```bash
# Build the binary (single binary, no cgo)
CGO_ENABLED=0 go build -o tgnas ./cmd/tgnas

# Run the combined S3 + WebDAV server (reads data/config.yaml by default)
./tgnas
./tgnas -c path/to/config.yaml          # short alias for -config
./tgnas -debug                          # debug logging to stderr (must precede subcommand)
./tgnas s3 | ./tgnas dav                # single-protocol modes

# Local read-only CLI against the SQLite metadata (no Telegram, no HTTP)
./tgnas ls  [-n|-limit N] bucket[/prefix]
./tgnas lsd [bucket[/prefix]]
./tgnas bucket rename [--dry-run] old new

# Tests
go test ./...                           # all packages
go test ./store/...                     # one package tree
go test -run TestCompleteMultipart ./store   # one test
go test -race ./...                     # with race detector

# Docker
docker build -t tgnas .
docker run -p 9000:9000 -v "$PWD/data:/app/data" \
  -e TGNAS_SECRET_KEY=... -e TGNAS_TELEGRAM_BOT_TOKEN=... -e TGNAS_TELEGRAM_CHAT_ID=... tgnas
```

The `data/config.yaml` resolved path is also where SQLite is created (`data/metadata.sqlite` by default). Tests use temp dirs; SQLite files are gitignored.

## Architecture

### Request flow
`cmd/tgnas/main.go` wires everything in `runServiceWithDebug`:

1. `config.LoadFile` reads YAML, validates bot token format, resolves env-var interpolation in `chat_id` / secrets / listen address / sqlite path.
2. `metadata.OpenSQLite` opens the database; configured buckets are `UpsertBucket`'d and any previously-known buckets not in config are `DisableBucketsExcept`'d (they become orphans, not deleted).
3. `telegram.NewHTTPClient` and `store.NewObjectStore` are constructed. The store owns concurrency semaphores for uploads/downloads/Telegram calls and a `KeyedLocker` for per-key serialization.
4. `s3api.NewServer` and `dav.NewHandler` are built on top of the same `ObjectStore`. The two handlers are composed by `combinedHandler` (routes `/dav/*` to WebDAV, `/healthz`+`/readyz` to S3, everything else to S3).
5. `trustedProxyMiddleware` wraps the combined handler. If the remote IP matches `trusted_proxies` CIDRs **or** the forwarded host matches `trusted_proxy_hosts`, `X-Forwarded-Host` / `X-Forwarded-Proto` (or `Forwarded:` header) rewrite `r.Host` and `r.URL.Scheme` — this is required for SigV4 verification behind a reverse proxy.

### Storage layout
- `metadata/`: `Store` interface in `metadata/types.go` is the contract; SQLite implementation in `metadata/sqlite.go`. An `Object` row references one or more `Chunk` rows (chunked uploads). `RenameBucket` cascades across `buckets`/`objects`/`chunks` tables.
- `store/`: `ObjectStore` translates S3/WebDAV operations to metadata + Telegram calls.
  - `upload_strategy.go` picks the Telegram upload kind (`document` vs typed `photo`/`video`/`audio`/`animation`) using `Storage.UploadTypeStrategy` and `TypeSizeLimits`. Typed uploads can be recompressed by Telegram — we record the Bot API `file_size` rather than the original bytes, and ETags for typed uploads are non-verifiable.
  - Large objects are split into chunks (`Storage.EnableChunking`, `MaxFileSize`, `ChunkSize`). Each chunk is a separate Telegram message.
  - Multipart upload state is held **in memory** on `ObjectStore.multipartUploads` (capped by `maxMultipartUploads`). Restarting the server drops in-flight uploads.
  - `locks.go` provides per-key locking so concurrent PUT/DELETE on the same key serialize.
- `telegram/`: thin wrapper over Bot API multipart upload + `getFile` download. `caption.go` formats the configured `caption_template`.

### Protocol surfaces
- `internal/s3api/`: SigV4 verification (header + query/presigned), XML responses, listing pagination via opaque `list_token`, multipart endpoints, public-read buckets allow anonymous `GET`/`HEAD` only. The `ObjectStore` interface at the top of `server.go` is what S3 needs from the store — keep it satisfied when adding store methods.
- `internal/dav/`: built on `golang.org/x/net/webdav`. `fs.go` adapts the metadata store to `webdav.FileSystem`. `MKCOL` writes a zero-byte directory-marker object (key ending in `/`). `COPY`/`MOVE` within the same bucket are metadata-only — they reuse existing chunk rows rather than re-uploading. `LOCK`/`UNLOCK` return not-implemented via `noLockSystem` (use pointer receivers — see `9b76462`).

### Config quirks worth knowing
- `chat_id: "${VAR}"` is **full-string interpolation only** — partial like `prefix-${VAR}` is not supported. Empty resolved value fails validation.
- The WebDAV `prefix` is normalized to have a trailing `/`, cannot be `/`, and cannot collide with the first path segment of any configured bucket name.
- A bucket present in SQLite but missing from config is an **orphan**: object access is forbidden, but `DELETE /{bucket}` (S3) or `DELETE /dav/{bucket}` (WebDAV) cleans it up.

## Testing notes

- `internal/testutil/faketelegram.go` is the in-process fake used for integration tests. Prefer it over mocking individual telegram methods.
- `cmd/tgnas/main_test.go` exercises CLI subcommands by overriding `runServiceFunc`, `newObjectStore`, and `listenAndServe` package vars — keep these vars when refactoring.
- SigV4 tests in `internal/s3api/sigv4_test.go` lock the clock via `WithSigV4Clock`; do the same in any new signature tests.

## Design docs

Larger feature designs live under `docs/superpowers/specs/` (proposal) and `docs/superpowers/plans/` (implementation plan). When extending multipart upload, WebDAV, public-read, bucket rename, or the CLI, read the corresponding spec first.
