# Expansion Plan: iCloud-like Backend

This document outlines how to expand the repository to serve as a backend for an iCloud-like interface, with clear coverage of upload (single/multiple), delete (single/multiple), and read (single/multiple).

---

## 1. Current State vs. Desired Capabilities

| Capability | Current | Desired | Status |
|------------|---------|---------|--------|
| **Upload single** (photo/video) | ✅ PUT `/api/v1/storage/files/{path}`, POST `/api/v1/storage/files/raw` | Same | Done |
| **Upload multiple** | ✅ POST `/api/v1/storage/files` (multipart) | Same | Done |
| **Delete one** | ❌ | New | **To implement** |
| **Delete many** | ❌ | New | **To implement** |
| **Read one file** | ✅ GET `/api/v1/storage/files/{path}` | Same | Done |
| **Read multiple files** | ✅ POST `/api/v1/storage/files/read` (JSON body) | Same | Done |

**Conclusion:** The only missing core feature is **delete** (single + batch). The rest of the plan focuses on adding delete and optional enhancements that make the backend more “iCloud-like” (listing, metadata, etc.).

---

## 2. Delete Feature Design

### 2.1 API Shape

- **Delete single**
  - `DELETE /api/v1/storage/files/{filePath}`
  - Response: `204 No Content` on success, or `404` if object does not exist (optional: body with error message).

- **Delete multiple**
  - `POST /api/v1/storage/files/delete` (or `DELETE` with body; many clients support `DELETE` + body)
  - Request body: `{ "file_paths": ["path/a", "path/b", ...] }`
  - Response: JSON with list of deleted paths and list of errors (same pattern as `WriteFiles` / `ReadFiles`), e.g.:
    - `{ "deleted": ["path/a"], "errors": [{ "file_path": "path/b", "error": "not found" }] }`
  - HTTP status: `200 OK` (partial success) or `204` when all deleted and no errors (optional).

Recommendation: use **DELETE single** by path and **POST …/delete** for batch to avoid relying on `DELETE` with body.

### 2.2 Layer Changes (summary)

| Layer | Change |
|-------|--------|
| **Storage interface** (`internal/service/storage.go`) | Add `DeleteFile(ctx, path)` and `DeleteFiles(ctx, paths)` (or a single `DeleteFiles` that accepts one or many). |
| **GCS storage** (`internal/infrastructure/gcs/storage.go`) | Implement delete using GCS `Object.Delete(ctx)`. Batch = loop and collect successes/errors. |
| **Service** (`internal/service/storage_service.go`) | Add `DeleteFile` and `DeleteFiles` that delegate to storage. |
| **Handler** (`internal/handler/storage_handler.go`) | Add `DeleteFile` (for `DELETE /api/v1/storage/files/{path}`) and `DeleteFiles` (for `POST /api/v1/storage/files/delete`). Wire in `SetupRoutes`. |

No change to config or GCS client is required for delete.

---

## 3. Optional Enhancements (iCloud-like)

These are not required for the “upload / delete / read single and multiple” set but improve the product.

### 3.1 List files (folder / prefix)

- **Endpoint:** e.g. `GET /api/v1/storage/files?prefix=photos/&delimiter=/` (or `GET /api/v1/storage/list?prefix=...`).
- **Response:** List of object names (and optionally sizes, content types, last modified) under the prefix, plus optional “folders” (common prefixes) if using delimiter.
- **Layers:** Add `List(ctx, prefix, delimiter)` to storage interface; implement in GCS with `Bucket.Objects(ctx, prefix)`; add service + handler and route.

Useful for: gallery view, “recent”, “folder” navigation.

### 3.2 Metadata in responses

- **Already there:** `ReadFile` / `ReadFiles` return name, content type, size.
- **Optional:** Add creation time, last modified (GCS `attrs.Created, attrs.Updated`) to `FileMetadata` and to write/read responses so the UI can show “date taken” / “last modified” like iCloud.

### 3.3 User / tenant isolation

- **Idea:** All paths are prefixed by `user_id` (or tenant id), e.g. `users/{user_id}/photos/...`.
- **Implementation:** Middleware that resolves user (e.g. from JWT or API key) and injects path prefix; handlers only ever see “logical” paths that already include the prefix, or the middleware rewrites paths. No change to storage interface if prefix is applied before calling storage.

Important if multiple users share the same bucket.

### 3.4 Streaming and large files

- **Current:** Entire file is read into memory (`io.ReadAll` in GCS storage). Fine for small photos/videos; can be an issue for very large files.
- **Improvement:** For `ReadFile`, support streaming: write `io.Copy(w, reader)` from GCS reader to response writer, and set `Content-Length` from object attrs. Same for single-file download; multi-file read can stay as JSON + base64 or be limited to smaller items.

### 3.5 Authentication and rate limiting

- **Auth:** Add middleware (e.g. JWT or API key) in front of `/api/v1/...` so only authenticated clients can upload/delete/read.
- **Rate limiting:** Optional per-user or per-IP limits to avoid abuse.

---

## 4. Recommended Implementation Order

1. **Phase 1 – Delete (required)**  
   - Add `DeleteFile` and `DeleteFiles` to storage interface and GCS implementation.  
   - Add service methods and handlers.  
   - Wire routes: `DELETE /api/v1/storage/files/{path}`, `POST /api/v1/storage/files/delete`.  
   - Add/update tests and README/API docs.

2. **Phase 2 – Polish and docs**  
   - Document all endpoints (OpenAPI/Swagger or README table).  
   - Optionally add list endpoint and metadata (created/updated) in responses.

3. **Phase 3 – Scale and security**  
   - Streaming for single-file read.  
   - User/tenant path prefix and auth middleware.  
   - Rate limiting if needed.

---

## 5. File-by-File Changes for Phase 1 (Delete)

| File | Change |
|------|--------|
| `internal/service/storage.go` | Add `DeleteFile(ctx, path) error` and `DeleteFiles(ctx, paths) (*DeleteResponse, error)`; define `DeleteResponse` (e.g. `Deleted []string`, `Errors []DeleteError`) and `DeleteError`. |
| `internal/infrastructure/gcs/storage.go` | Implement `DeleteFile` (single `Object.Delete`), `DeleteFiles` (loop, collect successes and errors). |
| `internal/service/storage_service.go` | Add `DeleteFile(ctx, path)` and `DeleteFiles(ctx, paths)` calling storage. |
| `internal/handler/storage_handler.go` | Add `DeleteFile(w, r)` for `DELETE /.../files/{path}` and `DeleteFiles(w, r)` for `POST /.../files/delete`; in `SetupRoutes` register both. |
| `internal/service/storage_contract_test.go` | If you have interface mocks or integration tests, extend them for delete. |
| `internal/service/storage_service_test.go` | Add tests for delete service methods. |
| `README.md` (or API doc) | Document delete endpoints and request/response. |

No new packages are required; delete stays within the existing handler → service → storage flow.

---

## 6. Summary

- **Must-have for “iCloud-like” backend:** Add **delete one** and **delete many** (Phase 1). Upload and read (single and multiple) are already in place.
- **Nice-to-have:** List by prefix, metadata (created/updated), streaming for large files, user-scoped paths, auth, and rate limiting (Phases 2–3).

Following the file-by-file plan above will give you a clear, testable expansion path without changing the existing architecture.
