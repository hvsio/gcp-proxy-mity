# GCP Proxy Mity

A Go Cloud Run service that exposes GCS-backed storage APIs and first-party photo-library APIs for Uni Album. The storage boundary is provider-neutral so the backing implementation can be swapped later without changing the HTTP API.

## Features

- Stream a single file from cloud storage with `GET /api/v1/storage/files/{path}`.
- Return metadata for `HEAD /api/v1/storage/files/{path}` without a response body.
- List file metadata with `GET /api/v1/storage/files?prefix={optional-prefix}`.
- Read multiple files with `POST /api/v1/storage/files/read`.
- Read all images under a prefix with `POST /api/v1/storage/files/read`.
- Optional IAP JWT validation for protected deployments.
- Optional Google ID-token validation for browser clients using the configured single owner allowlist.
- Optional CORS allowlist.
- Optional Cloud SQL/Postgres wiring and migrations for metadata features.
- Photo-library APIs for assets, albums, favorite state, upload, signed media URLs, and background job status.

## Project Structure

```text
cmd/server/                  # Application entry point
internal/auth/               # IAP and Google ID-token validation
internal/config/             # Environment configuration
internal/httpapi/            # HTTP routes, handlers, CORS
internal/storage/            # Read-only storage abstraction and GCS adapter
internal/platform/database/  # Postgres and Cloud SQL infrastructure
terraform/                   # GCP infrastructure
test/acceptance/             # Optional real-GCS acceptance tests
```

## Configuration

```bash
PORT=8080
GCP_PROJECT_ID=your-project-id
GCS_BUCKET_NAME=your-bucket-name
GOOGLE_APPLICATION_CREDENTIALS=base64-encoded-service-account-json
SIGNED_URL_SERVICE_ACCOUNT_EMAIL=gcp-proxy-mity-app@your-project-id.iam.gserviceaccount.com

ENABLE_DATABASE=false
DB_TYPE=cloudsql
DB_INSTANCE_CONNECTION_NAME=
DB_HOST=localhost
DB_PORT=5432
DB_DATABASE_NAME=gcp_proxy
DB_USERNAME=gcp_proxy_app
DB_PASSWORD=
DB_SSL_MODE=disable

IAP_AUDIENCE=
GOOGLE_OAUTH_CLIENT_ID=
ALLOWED_IAP_EMAILS=
CORS_ALLOWED_ORIGINS=
```

When running on GCP with workload identity, leave `GOOGLE_APPLICATION_CREDENTIALS` empty.
Database settings are only required when `ENABLE_DATABASE=true`.
`SIGNED_URL_SERVICE_ACCOUNT_EMAIL` must be set for `/api/v1/assets/{id}/urls`; the runtime service account needs permission to sign blobs for that service account and create/read objects in the media bucket.
`ALLOWED_IAP_EMAILS` is reused as the single-owner allowlist for both IAP and browser Google ID-token validation.

## API

### Health

```http
GET /health
GET /ready
```

### Read one file

```http
GET /api/v1/storage/files/{path}
HEAD /api/v1/storage/files/{path}
```

Responses:

- `200 OK` with the file body for `GET`.
- `200 OK` with headers only for `HEAD`.
- `400 Bad Request` when the path is empty.
- `404 Not Found` when the storage provider reports a missing object.
- `504 Gateway Timeout` when the storage read times out.
- `500 Internal Server Error` for other storage failures.

### List files

```http
GET /api/v1/storage/files?prefix=photos/
```

Response:

```json
{
  "files": [
    {
      "name": "photos/a.jpg",
      "content_type": "image/jpeg",
      "size": 123,
      "updated_at": "2026-05-14T12:00:00Z"
    }
  ]
}
```

### Read multiple files

```http
POST /api/v1/storage/files/read
Content-Type: application/json

{
  "file_paths": ["photos/a.jpg", "photos/b.jpg"]
}
```

The response keeps successful files and per-file errors separate:

```json
{
  "files": [
    {
      "metadata": {
        "name": "photos/a.jpg",
        "content_type": "image/jpeg",
        "size": 123
      },
      "content": "base64-encoded-content"
    }
  ],
  "errors": [
    {
      "file_path": "photos/b.jpg",
      "error": "file not found"
    }
  ]
}
```

To read every image object under a bucket prefix, send a `prefix` without `file_paths`:

```http
POST /api/v1/storage/files/read
Content-Type: application/json

{
  "prefix": "photos/"
}
```

Only objects whose listed content type starts with `image/` are read.

### Photo library

```http
GET /api/v1/auth/session
GET /api/v1/assets?limit=100&cursor={optional-cursor}&albumId={optional-album-id}
POST /api/v1/assets/upload
GET /api/v1/assets/{id}
GET /api/v1/assets/{id}/urls
PATCH /api/v1/assets/{id}/favorite
GET /api/v1/albums
POST /api/v1/albums
PATCH /api/v1/albums/{id}
DELETE /api/v1/albums/{id}
POST /api/v1/albums/{id}/assets
DELETE /api/v1/albums/{id}/assets
GET /api/v1/jobs
GET /api/v1/status
```

Photo-library routes require either valid IAP identity or a browser Google ID token whose email is present in `ALLOWED_IAP_EMAILS`.
Media bytes remain private in GCS; the browser receives short-lived signed URLs only after the backend validates the requester.

## Development

```bash
go test ./...
go build -o bin/server ./cmd/server
```

If the local sandbox cannot write to the default Go caches:

```bash
GOCACHE=/tmp/gcp-proxy-mity-go-build \
GOMODCACHE=/tmp/gcp-proxy-mity-go-mod \
go test ./...
```

Acceptance tests compile and run with:

```bash
ACCEPTANCE_READ_FILE_PATH=path/in/bucket.txt \
ACCEPTANCE_EXPECTED_CONTENT='optional exact content' \
go test -tags=acceptance ./test/acceptance
```
