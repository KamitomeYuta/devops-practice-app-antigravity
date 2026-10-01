# ==============================================================================
# Variables & Default Configurations
# ==============================================================================
IMAGE_NAME     ?= devops-practice-app
IMAGE_TAG      ?= local
IMAGE          := $(IMAGE_NAME):$(IMAGE_TAG)
CONTAINER_NAME ?= devops-practice-app
PORT           ?= 8080

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# ==============================================================================
# Targets
# ==============================================================================

.PHONY: help
help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## Run go mod tidy
	@echo "==> Tidying Go module dependencies..."
	go mod tidy

.PHONY: test
test: ## Run unit tests with race detection and coverage output
	@echo "==> Running unit tests with race detection and coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "==> Coverage summary:"
	go tool cover -func=coverage.out

.PHONY: lint-docker
lint-docker: ## Run hadolint against Dockerfile via local CLI or Docker container
	@echo "==> Linting Dockerfile with Hadolint..."
	@if command -v hadolint >/dev/null 2>&1; then \
		hadolint Dockerfile; \
		echo "✓ Hadolint passed with 0 warnings"; \
	else \
		echo "hadolint not found locally, executing via container..."; \
		docker run --rm -i hadolint/hadolint:latest-alpine hadolint - < Dockerfile; \
		echo "✓ Hadolint passed with 0 warnings"; \
	fi

.PHONY: build
build: ## Build the container image with tag devops-practice-app:local
	@echo "==> Building container image $(IMAGE)..."
	DOCKER_BUILDKIT=1 docker build -t $(IMAGE) .
	@echo "✓ Image $(IMAGE) successfully built"

.PHONY: scan
scan: ## Run trivy vulnerability scan on devops-practice-app:local (HIGH,CRITICAL)
	@echo "==> Scanning container image $(IMAGE) for HIGH/CRITICAL vulnerabilities..."
	@if command -v trivy >/dev/null 2>&1; then \
		trivy image --skip-version-check --severity HIGH,CRITICAL --exit-code 1 $(IMAGE); \
	else \
		echo "trivy not found locally, executing via container..."; \
		docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:latest image --skip-version-check --severity HIGH,CRITICAL --exit-code 1 $(IMAGE); \
	fi

.PHONY: run
run: ## Run the container on port 8080 locally as non-root user
	@echo "==> Running $(IMAGE) on port $(PORT) as non-root user (10001:10001)..."
	docker run --rm \
		--name $(CONTAINER_NAME) \
		-p $(PORT):8080 \
		--user 10001:10001 \
		$(IMAGE)

.PHONY: stop
stop: ## Stop the locally running container
	@echo "==> Stopping container $(CONTAINER_NAME)..."
	@docker stop $(CONTAINER_NAME) 2>/dev/null || true
