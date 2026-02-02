# Project TODO: Initial Stages → Production Ready

Track of tasks to take the iCloud-like storage backend from its current state to production ready.  
See [docs/EXPANSION_PLAN.md](docs/EXPANSION_PLAN.md) for detailed design.

---

## Milestone: Core API (CRUD)

- [x] Upload single file (photo/video) — PUT `/api/v1/storage/files/{path}`, POST `/api/v1/storage/files/raw`
- [x] Upload multiple files — POST `/api/v1/storage/files` (multipart)
- [x] Read single file — GET `/api/v1/storage/files/{path}`
- [x] Read multiple files — POST `/api/v1/storage/files/read`
- [x] Delete single file — DELETE `/api/v1/storage/files/{path}`
- [x] Delete multiple files — POST `/api/v1/storage/files/delete`

---

## Milestone: API Enhancements (iCloud-like)

- [ ] List files by prefix — e.g. GET `/api/v1/storage/files?prefix=photos/&delimiter=/` (folder-style listing)
- [ ] Expose metadata in responses — creation time, last modified (e.g. in `FileMetadata` and read/write responses)
- [ ] Streaming for single-file read — stream large files instead of loading into memory (`io.Copy` from GCS reader)
- [ ] Optional: pagination or max results for list endpoint

---

## Milestone: Security & Auth

- [ ] Authentication — middleware for JWT or API key on `/api/v1/*`
- [ ] User/tenant isolation — path prefix per user (e.g. `users/{user_id}/...`) from auth context
- [ ] Rate limiting — per-user or per-IP limits to prevent abuse
- [ ] Input validation & sanitization — path traversal prevention, max path length, allowed characters
- [ ] CORS configuration — explicit allowed origins for frontend
- [ ] HTTPS only in production — redirect or enforce TLS

---

## Milestone: Observability & Operations

- [ ] Structured logging — consistent log format (e.g. JSON), request IDs, error codes
- [ ] Request/response logging — optional debug logging for API calls (without sensitive data)
- [ ] Health check enrichment — e.g. dependency checks (GCS reachability) in `/health`
- [ ] Metrics — request count, latency, error rate (e.g. Prometheus or OpenCensus)
- [ ] Tracing — distributed tracing for requests (e.g. Cloud Trace)
- [ ] Graceful shutdown — already present; verify all in-flight requests drain

---

## Milestone: Deployment & DevOps

- [ ] CI pipeline — run tests and linters on PR/push (e.g. build, `go test`, `go vet`, staticcheck)
- [ ] Deploy workflow — current workflow deploys to Cloud Run; ensure env/secrets and rollback strategy are documented
- [ ] Environment parity — dev/staging/production config and `.env.example` kept in sync
- [ ] Secrets management — no credentials in repo; document use of Secret Manager or env vars
- [ ] Dockerfile optimization — multi-stage build, minimal image, non-root user if not already
- [ ] Infrastructure as Code — optional: Terraform/Pulumi for bucket, IAM, Cloud Run

---

## Milestone: Documentation

- [ ] API reference — OpenAPI/Swagger spec or hosted docs for all endpoints
- [ ] README — deployment, env vars, and local run instructions (expand as needed)
- [ ] Runbook — operational procedures (scaling, incidents, rollback)
- [ ] Architecture decision records (ADRs) — optional: document key design choices

---

## Milestone: Testing & Quality

- [ ] Unit tests — storage and service layers covered; extend handler tests for new endpoints
- [ ] Integration tests — optional: tests against real GCS (emulator or test bucket)
- [ ] E2E or smoke tests — optional: hit live API after deploy
- [ ] Load/performance testing — optional: establish baselines for upload/download/list
- [ ] Dependency updates — Dependabot or similar; keep Go and libs current

---

## Milestone: Performance & Scale

- [ ] Streaming for large uploads — avoid loading full body into memory for very large files
- [ ] Connection pooling / timeouts — GCS client and HTTP server timeouts tuned
- [ ] Max request size — document and enforce limits (e.g. 100MB for uploads)
- [ ] List endpoint performance — cursor/pagination if listing large buckets

---

*Last updated: project state after Phase 1 (delete) implementation.*
