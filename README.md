# DevOps Practice App (Go REST API)

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Multi--stage-2496ED?logo=docker&logoColor=white)](Dockerfile)
[![Security](https://img.shields.io/badge/Security-Non--Root%20(10001)-green.svg)](Dockerfile)

A production-ready, minimal, cloud-native REST API built with Go (1.23+) designed for Kubernetes, containerized microservice architectures, and modern DevSecOps practices.

---

## Features

- **Standard Library Routing:** Uses Go 1.22+ enhanced `net/http.ServeMux` with method routing (`GET /healthz`), eliminating third-party routing dependencies.
- **Kubernetes Probes:**
  - `GET /healthz`: Liveness probe returning `{"status": "healthy"}`.
  - `GET /ready`: Readiness probe returning `{"status": "ready"}`.
- **Service Metadata:**
  - `GET /api/v1/info`: Application version, environment name (`APP_ENV`), and pod/host name (`os.Hostname()`).
- **Resilient HTTP Server:** Enforces timeout safeguards (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`) to prevent resource exhaustion and Slowloris attacks.
- **Graceful Shutdown:** Intercepts `SIGINT` and `SIGTERM` signals with a 10-second timeout context to drain active in-flight connections cleanly.
- **Observability:** Structured JSON logging using standard library `log/slog` recording request method, path, remote IP, status code, latency, and User-Agent.
- **Hardened Container Image:**
  - Multi-stage build (`golang:1.23-alpine` -> `alpine:3.20`).
  - Runs as dedicated non-root user (`appuser:appuser`, UID/GID `10001:10001`).
  - Zero packages installed in the runtime layer.
  - Stripped binary (`-ldflags="-s -w"`, `-trimpath`) with CGO disabled.
  - Total image size is only **~21 MB**.
  - Verified 0 warnings on `hadolint`.

---

## API Endpoints

| Method | Path | Status | Response | Purpose |
| :--- | :--- | :--- | :--- | :--- |
| `GET` | `/healthz` | `200 OK` | `{"status":"healthy"}` | Kubernetes Liveness Probe |
| `GET` | `/ready` | `200 OK` | `{"status":"ready"}` | Kubernetes Readiness Probe |
| `GET` | `/api/v1/info` | `200 OK` | `{"version":"...","environment":"...","hostname":"..."}` | Service metadata & runtime information |

> [!NOTE]
> All other HTTP methods (e.g. `POST`, `PUT`, `DELETE`) on these endpoints will automatically return HTTP `405 Method Not Allowed`.

---

## Configuration

Configuration is loaded strictly from environment variables according to 12-Factor App methodology:

| Variable | Default Value | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | Port for the HTTP server to listen on |
| `APP_ENV` | `development` | Runtime environment name (`development`, `staging`, `production`) |
| `APP_VERSION` | `1.0.0` | Application version (fallback to build ldflags) |
| `SHUTDOWN_TIMEOUT_SECONDS` | `10` | Timeout duration in seconds for graceful server shutdown |

---

## Local Development & Makefile

A self-documenting [`Makefile`](Makefile) is provided for convenience:

```bash
# View all available make targets
make help

# Tidy Go dependencies
make tidy

# Run unit tests with race detector and statement coverage
make test

# Lint Dockerfile using hadolint (CLI or Docker fallback)
make lint-docker

# Build container image (devops-practice-app:local)
make build

# Run Trivy vulnerability scan (severity: HIGH, CRITICAL)
make scan

# Run container locally on port 8080 as non-root user
make run

# Stop locally running container
make stop
```

---

## Building & Running Manually

### Running with Go
```bash
# Run locally
go run .

# Run with custom environment variables
PORT=9090 APP_ENV=staging go run .
```

### Building Binary with Version Injection
```bash
go build -ldflags="-s -w -X main.Version=1.2.0" -o server .
./server
```

### Docker
```bash
# Build Docker image
docker build -t devops-practice-app:latest .

# Run container
docker run --rm -p 8080:8080 --user 10001:10001 devops-practice-app:latest
```

---

## Security & Verification

This project has been verified with:
- **Go Race Detector:** `go test -v -race ./...` (0 data races detected)
- **Go Vet:** `go vet ./...` (0 issues)
- **Hadolint:** `hadolint Dockerfile` (0 warnings, compliant with CIS benchmarks)
- **Trivy Scanner:** `trivy fs --scanners secret,vuln .` (0 exposed secrets detected)
