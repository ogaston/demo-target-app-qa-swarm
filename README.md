# demo-target-app-qa-swarm

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.0.3-6BA539?style=flat&logo=openapi-initiative)](/openapi.json)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Reference target application for [Agentic QA Swarm](https://github.com/ogaston/agentic-qa-swarm): a transactional orders and inventory management microservice built in Go.

It serves as the test subject for autonomous QA swarms, featuring:
- A deterministic baseline state and automated reset contract (`baseline.sh`).
- Seeded bug injection (`TARGET_APP_BUG=oversell`) for validating defect detection.
- Complete OpenAPI 3.0 specification and embedded lightweight Swagger UI.
- Dual-listener architecture: public business API on `:80` and isolated admin/baseline listener on `:9090`.

---

## Architecture Overview

```
                      +---------------------------------------+
                      |       demo-target-app-qa-swarm        |
                      |                                       |
  [Browser / Users]   |   +-------------------------------+   |
-------- :80 -------->|-->| Public API (appMux)           |   |
  [QA Agents]         |   | - GET  /swagger, /docs        |   |
                      |   | - GET  /openapi.json          |   |
                      |   | - GET  /health, /             |   |
                      |   | - CRUD /products, /orders     |   |
                      |   +---------------+---------------+   |
                      |                   |                   |
                      |                   v                   |
                      |          [PostgreSQL / Memory]        |
                      |                   ^                   |
                      |                   |                   |
  [go-reset]          |   +---------------+---------------+   |
-------- :9090 ------>|-->| Baseline API (baselineMux)    |   |
  (Bearer Token)      |   | - POST /__aqs/baseline/clean  |   |
                      |   | - GET  /__aqs/baseline/verify |   |
                      |   | - GET  /__aqs/baseline/version|   |
                      |   +-------------------------------+   |
                      +---------------------------------------+
```

---

## Interactive Documentation (Swagger UI)

The service includes an embedded, lightweight Swagger UI. It does not add heavy Go dependencies to the binary and directly renders the application's `/openapi.json` via CDN.

- **Swagger UI:** `http://localhost:8080/swagger` (or `/swagger/`)
- **Docs Redirect:** `http://localhost:8080/docs` &rarr; redirects to `/swagger`
- **Raw OpenAPI 3.0 Spec:** `http://localhost:8080/openapi.json`

> **Note:** The `/swagger`, `/docs`, `/openapi.json`, and `/` endpoints bypass the internal database gate, ensuring API documentation and probes remain accessible even when the database is initializing or offline.

---

## Business Invariants & Seeded Defect

### Invariant
> **Stock is never negative:** A product's available quantity must always be &ge; 0.

### Seeded Defect Simulation
The app provides a controlled bug toggle via the `TARGET_APP_BUG` environment variable:

| Mode | `TARGET_APP_BUG` | Behavior on `POST /orders` |
| :--- | :--- | :--- |
| **Healthy (Default)** | *(unset)* | Checks available stock before deducting. If `qty > stock`, the order is rejected with `409 Conflict` (`insufficient_stock`). |
| **Defect (Oversell)** | `oversell` | Skips stock validation. The inventory is unconditionally decremented (`UPDATE products SET stock = stock - qty`), allowing stock to drop below 0. Used by QA agents to detect concurrency and inventory defects. |

---

## API Reference

### 1. Public Application API (`LISTEN_ADDR`, default `:80`)

| Method | Path | Summary | Description / Response |
| :--- | :--- | :--- | :--- |
| `GET` | `/` | Service Info | Returns service identity and version (`{"service":"target-app","version":"ref-v1"}`). |
| `GET` | `/swagger` | Swagger UI | Interactive API documentation. |
| `GET` | `/docs` | Docs Alias | 301 Permanent Redirect to `/swagger`. |
| `GET` | `/openapi.json` | OpenAPI Spec | Raw OpenAPI 3.0.3 JSON schema. |
| `GET` | `/health` | Health Check | Returns `200 OK` (`{"status":"ok"}`) if DB is connected, or `503 Service Unavailable` (`{"status":"db_down"}`). |
| `GET` | `/products` | List Products | Returns all products (`[{"id":1,"name":"cafe","stock":10}, ...]`). |
| `POST` | `/products` | Create Product | Body: `{"name":"string", "stock":int}`. Returns `201 Created` with new product. |
| `GET` | `/orders` | List Orders | Returns order history (`[{"id":1,"product_id":1,"qty":2,"created_at":"..."}, ...]`). |
| `POST` | `/orders` | Place Order | Body: `{"product_id":int, "qty":int}`. Deducts stock atomically and returns `201 Created` or `409 Conflict`. |

### 2. Baseline Admin API (`BASELINE_LISTEN_ADDR`, default `:9090`)

Protected by `Authorization: Bearer <token>` where `<token>` is read from `BASELINE_TOKEN_FILE`.

| Method | Path | Summary | Description |
| :--- | :--- | :--- | :--- |
| `POST` | `/__aqs/baseline/clean` | Reset Database | Truncates `orders` and `products`, resets identity sequences, and re-inserts baseline seed (`cafe: 10`, `te: 5`). Returns `204 No Content`. |
| `GET` | `/__aqs/baseline/verify` | Verify Baseline | Returns plain text count of differing rows (`0` indicates database is clean). |
| `GET` | `/__aqs/baseline/version` | Baseline Version | Returns current baseline protocol version (`ref-v1`). |

---

## Configuration Reference

Configuration is supplied via environment variables. The service fails closed at boot if any required variable is missing or invalid.

| Variable | Default | Required | Description |
| :--- | :--- | :---: | :--- |
| `DATABASE_URL` | — | **Yes**\* | PostgreSQL connection DSN (e.g., `postgres://user:pass@localhost:5432/dbname?sslmode=disable`). |
| `TARGET_APP_STORE` | `""` | No | Set to `memoria` to use an in-memory store for integration testing without PostgreSQL. |
| `BASELINE_TOKEN_FILE` | — | **Yes** | Absolute path to file containing the baseline admin authorization token. |
| `LISTEN_ADDR` | `:80` | No | Bind address for public HTTP traffic. |
| `BASELINE_LISTEN_ADDR` | `:9090` | No | Bind address for baseline reset/admin traffic. |
| `TARGET_APP_BUG` | `""` | No | Set to `oversell` to enable the seeded defect; leave unset for the healthy app. |

*\* `DATABASE_URL` must be omitted when `TARGET_APP_STORE=memoria` (in-memory mode explicitly forbids a database URL).*

---

## Quickstart & Local Execution

### Option A: In-Memory Mode (Zero Database Setup)

Ideal for quick development, running tests, or viewing the Swagger documentation locally without starting PostgreSQL:

```bash
# 1. Create a temporary token file
echo "dev-token-12345" > /tmp/baseline.token

# 2. Start the service with in-memory store
TARGET_APP_STORE=memoria \
BASELINE_TOKEN_FILE=/tmp/baseline.token \
LISTEN_ADDR=:8080 \
BASELINE_LISTEN_ADDR=:9090 \
go run .
```

- Open Swagger UI in your browser: [http://localhost:8080/swagger](http://localhost:8080/swagger)
- View OpenAPI JSON: [http://localhost:8080/openapi.json](http://localhost:8080/openapi.json)

---

### Option B: Local PostgreSQL with Docker

1. **Start PostgreSQL:**
   ```bash
   docker run --name target-db -e POSTGRES_PASSWORD=secret -e POSTGRES_DB=targetdb -p 5432:5432 -d postgres:16-alpine
   ```

2. **Create baseline token file:**
   ```bash
   echo "admin-secret-token" > /tmp/baseline.token
   ```

3. **Run Target App:**
   ```bash
   DATABASE_URL="postgres://postgres:secret@localhost:5432/targetdb?sslmode=disable" \
   BASELINE_TOKEN_FILE=/tmp/baseline.token \
   LISTEN_ADDR=:8080 \
   BASELINE_LISTEN_ADDR=:9090 \
   go run .
   ```

---

### Option C: Docker Container Build

The `Dockerfile` employs a multi-stage Alpine build, creating a lightweight image running as non-root user `65532:65532` with `cap_net_bind_service`:

```bash
# Build the container
docker build -t demo-target-app:local .

# Run container with in-memory store
docker run --rm -p 8080:80 -p 9090:9090 \
  -e TARGET_APP_STORE=memoria \
  -e BASELINE_TOKEN_FILE=/etc/token \
  -v /tmp/baseline.token:/etc/token:ro \
  demo-target-app:local
```

---

## Testing & Verification

### Running Automated Tests
```bash
GOTOOLCHAIN=auto go test -v ./...
```

### Testing the Baseline Script
The `baseline/baseline.sh` script communicates with `:9090` to execute the platform reset contract:

```bash
export TARGET_APP_BASELINE_URL="http://localhost:9090"
export TARGET_APP_BASELINE_TOKEN_FILE="/tmp/baseline.token"

# Check baseline version (prints: ref-v1)
./baseline/baseline.sh version

# Verify diff count (prints: 0 when clean)
./baseline/baseline.sh verify

# Reset database to baseline
./baseline/baseline.sh clean
```

---

## Platform Contract (Agentic QA Swarm)

This repository fulfills the platform contract defined in `docs/operaciones/app-objetivo.md` of `agentic-qa-swarm`:

1. **OpenAPI Specification:** `GET /openapi.json` returns valid OpenAPI 3.0.3, enabling `go-warm-manager` to discover endpoints and request schemas.
2. **Baseline Protocol:** `baseline/baseline.sh` provides `clean`, `verify`, and `version` commands consumed by `go-reset`.
3. **CI/CD Image Publishing:** Every push to `main` builds and pushes an image tagged with the commit SHA (`ghcr.io/ogaston/demo-target-app-qa-swarm:<commit sha>`). The container package is public so Kubernetes clusters can pull without imagePullSecrets.
