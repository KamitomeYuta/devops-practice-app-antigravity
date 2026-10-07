# syntax=docker/dockerfile:1

# ==========================================
# Stage 1: Build & Security Hardening
# ==========================================
FROM golang:1.27-alpine AS builder

WORKDIR /src

# Copy dependency manifests first for optimal layer caching
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY main.go ./

# Compile statically linked binary with debug symbols stripped
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/server .

# ==========================================
# Stage 2: Minimal & Hardened Runtime
# ==========================================
FROM alpine:3.20

# Open Container Initiative (OCI) image specifications
LABEL org.opencontainers.image.title="devops-practice-app" \
      org.opencontainers.image.description="Production-hardened minimal Go REST API with health probes" \
      org.opencontainers.image.version="1.0.0" \
      org.opencontainers.image.authors="DevSecOps Team <devsecops@example.com>" \
      org.opencontainers.image.vendor="Example Org" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/example/devops-practice-app"

# Set explicit working directory
WORKDIR /app

# Create dedicated non-privileged user and group (UID/GID 10001) without installing external packages
RUN addgroup -g 10001 -S appuser && \
    adduser -u 10001 -S appuser -G appuser

# Copy CA certificates from builder stage for secure TLS calls
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary from builder with strict ownership
COPY --from=builder --chown=10001:10001 /app/server /app/server

# Switch to non-privileged user before EXPOSE
USER 10001:10001

# Document container port
EXPOSE 8080

# Production environment variables
ENV PORT=8080 \
    APP_ENV=production

# Execute binary
ENTRYPOINT ["/app/server"]
