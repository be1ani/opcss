# OPC Storage Service

<p align="center">
  <img src="assets/opcss.png" alt="OPC Storage Service" width="200"/>
</p>

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)
![Azure](https://img.shields.io/badge/Azure_Blob_Storage-primary-0078D4?logo=microsoftazure&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-green)
![CI](https://github.com/be1ani/opcfs/actions/workflows/ci.yml/badge.svg)

A cloud-native backend for managing **OPC-UA / OPC-DA industrial files** with
chunked uploads, SHA-256 integrity verification, rich metadata, and pluggable
multi-cloud storage (Azure Blob, AWS S3, local disk).

---

## Architecture

```mermaid
graph TD
    Client["OPC Client / SCADA"] -->|REST / HTTPS| API["HTTP API :8080"]

    subgraph OPCStorageService
        API --> MW["Middleware\n(Logger · Recoverer)"]
        MW --> H["File Handlers\n(Chi router)"]
        H --> CS["pkg/checksum\nSHA-256 verification"]
        H --> Store["StorageBackend\ninterface"]
        H --> DB["db.Store\nPostgreSQL"]
    end

    Store --> Azure["Azure Blob Storage\n(minio-go SDK)"]
    Store --> S3["AWS S3"]
    Store --> Disk["Local Disk"]

    DB --> PG[("PostgreSQL 16")]
```

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.22 |
| HTTP framework | Chi v5 |
| Router | `github.com/go-chi/chi/v5` |
| Database | PostgreSQL 16 + sqlc |
| DB driver | `github.com/lib/pq` |
| Object storage | Azure Blob Storage via minio-go v7 |
| Integrity | SHA-256 (`pkg/checksum`) |
| Logging | `log/slog` (structured) |
| Observability | Prometheus + Grafana *(planned)* |
| Container | Docker (distroless runtime) |
| CI/CD | GitHub Actions |

---

## API

### `GET /healthz`

Returns service liveness.

```json
{ "status": "ok" }
```

### `POST /api/v1/files/{id}/chunks`

Upload a single chunk of a file. Chunks may arrive in any order.

**Request** — `multipart/form-data`

| Field | Type | Description |
|-------|------|-------------|
| `chunk_index` | int | Zero-based index of this chunk |
| `total_chunks` | int | Total number of chunks in the file |
| `chunk_data` | binary | Raw chunk bytes |

**Validation**
- `chunk_index` must be less than `total_chunks`
- File `{id}` must exist in the database

**Response** — `201 Created`

```json
{
  "chunk_index": 0,
  "storage_key": "abc123/chunks/0",
  "checksum": "b94d27b9..."
}
```

**Error responses**

```json
{ "error": "message" }
```

| Status | Condition |
|--------|-----------|
| `400` | Malformed form, invalid integers, or `chunk_index >= total_chunks` |
| `404` | File `{id}` not found |
| `500` | Storage or database failure |

When all chunks are received (`COUNT(chunks) == total_chunks`) the file record is automatically marked `status = complete`.

Storage key format: `{file_id}/chunks/{chunk_index}`

---

## Project Structure

```
opcfs/
├── cmd/server/          # Binary entrypoint; wires DB, storage, and router
├── internal/
│   ├── api/             # Chi router, handlers, shared writeJSON helper
│   │   ├── hander.go    # NewRouter, Handler struct, /healthz
│   │   └── chunks.go    # POST /api/v1/files/{id}/chunks
│   ├── config/          # Env-based config (LISTEN_ADDR, DATABASE_URL, STORAGE_*)
│   ├── db/              # PostgreSQL access
│   │   ├── db.go        # Store methods (GetFile, InsertChunk, CountChunks, MarkComplete)
│   │   ├── models.go    # File, Chunk, InsertChunkParams types
│   │   └── queries.sql  # sqlc query definitions
│   ├── middleware/       # Chain helper for composing net/http middleware
│   └── storage/         # StorageBackend interface + MinioBackend implementation
├── pkg/
│   └── checksum/        # SHA256Bytes / SHA256Reader helpers + unit tests
├── assets/              # Static assets (logo, etc.)
├── deploy/
│   ├── docker/          # Dockerfile (multi-stage)
│   └── grafana/         # Dashboard JSON exports
└── .github/workflows/   # CI/CD pipelines
```

---

## Database Schema

```sql
CREATE TABLE files (
    id         TEXT PRIMARY KEY,
    status     TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'complete'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chunks (
    id           BIGSERIAL PRIMARY KEY,
    file_id      TEXT NOT NULL REFERENCES files(id),
    chunk_index  INT  NOT NULL,
    total_chunks INT  NOT NULL,
    size         BIGINT NOT NULL,
    checksum     TEXT NOT NULL,  -- hex-encoded SHA-256
    storage_key  TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## Local Setup

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- `golangci-lint` (`brew install golangci-lint` or see [docs](https://golangci-lint.run/usage/install/))

### Run locally

```bash
# 1. Clone
git clone https://github.com/be1ani/opcfs.git
cd opcfs

# 2. Tidy dependencies
make tidy

# 3. Build
make build

# 4. Run
LISTEN_ADDR=:8080 \
DATABASE_URL=postgres://user:pass@localhost:5432/opcfs?sslmode=disable \
STORAGE_ENDPOINT=your-account.blob.core.windows.net \
STORAGE_ACCESS_KEY=your-access-key \
STORAGE_SECRET_KEY=your-secret-key \
STORAGE_BUCKET=your-container \
make run

# 5. Verify
curl http://localhost:8080/healthz
```

### Build Docker image

```bash
make docker-build
docker run -p 8080:8080 opcfs:latest
```

### Run tests & lint

```bash
make test
make lint
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | TCP address the HTTP server binds to |
| `DATABASE_URL` | — | PostgreSQL DSN (`postgres://user:pass@host/db`) |
| `STORAGE_ENDPOINT` | — | Azure Blob / S3-compatible endpoint host |
| `STORAGE_ACCESS_KEY` | — | Storage access key ID |
| `STORAGE_SECRET_KEY` | — | Storage secret access key |
| `STORAGE_BUCKET` | — | Bucket / container name |
| `STORAGE_USE_SSL` | `false` | Set `true` to enable TLS for storage connections |

---

## Contributing

1. Fork → feature branch → PR against `main`
2. All PRs must pass `make test` and `make lint`
3. Follow [Conventional Commits](https://www.conventionalcommits.org/)

---

## License

MIT © 2026 be1ani
