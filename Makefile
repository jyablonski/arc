.PHONY: build install clean test test-ci coverage coverage-full coverage-app coverage-html fmt lint help release mocks tidy deps

# Binary name
BINARY_NAME=arc

# Installation directory
INSTALL_DIR=$(HOME)/.local/bin

# Build directory
BUILD_DIR=bin

# Version (can be set via VERSION env var or detected from git)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# LDFLAGS for version injection
LDFLAGS = -X github.com/jyablonski/arc/cmd.version=$(VERSION)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the arc binary
	@echo "Building $(BINARY_NAME) (version: $(VERSION))..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install: build ## Build and install arc to ~/.local/bin
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)"
	@echo "Make sure $(INSTALL_DIR) is in your PATH"

uninstall: ## Remove arc from ~/.local/bin
	@echo "Removing $(BINARY_NAME) from $(INSTALL_DIR)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstalled $(BINARY_NAME)"

clean: ## Remove the built binary and coverage files
	@echo "Cleaning build artifacts..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -f coverage.out coverage.full.out coverage.app.out test-results.xml
	@echo "Clean complete"

test: ## Run tests with gotestsum (install: go install gotest.tools/gotestsum@latest)
	@echo "Running tests..."
	gotestsum --format testdox -- -race ./...

coverage-full:
	@echo "Generating coverage profile..."
	@go test -coverprofile=coverage.full.out ./...

# coverage.full.out must exist; strips *_moq.go lines into coverage.app.out and coverage.out
FILTER_MOQ_FROM_FULL = awk '/^mode:/ { print; next } /_moq\.go:/ { next } { print }' coverage.full.out > coverage.app.out && cp coverage.app.out coverage.out

test-ci: ## Run tests with coverage and JUnit output (for CI)
	@echo "Running tests with coverage..."
	gotestsum --format testdox --junitfile test-results.xml -- -race -coverprofile=coverage.full.out ./...
	@$(FILTER_MOQ_FROM_FULL)
	@echo ""
	@echo "Statement coverage (product code, *_moq.go excluded):"
	@go tool cover -func=coverage.out | tail -1
	@echo "coverage.out = filtered (same as coverage.app.out); coverage.full.out = unfiltered"

coverage: coverage-full ## Generate and display test coverage report (filtered; raw in coverage.full.out)
	@$(FILTER_MOQ_FROM_FULL)
	@echo ""
	@echo "Coverage by function (product code, moq stubs excluded):"
	@go tool cover -func=coverage.out
	@echo ""
	@echo "Coverage profile saved to coverage.out (filtered). Full: coverage.full.out"
	@echo "Run 'make coverage-html' to view HTML report"

coverage-app: coverage-full ## Short coverage summary (same filtering as coverage / test-ci)
	@$(FILTER_MOQ_FROM_FULL)
	@echo ""
	@go tool cover -func=coverage.out | tail -1
	@echo "coverage.out and coverage.app.out = filtered; coverage.full.out = raw"

coverage-html: coverage ## Generate and open HTML coverage report (uses filtered coverage.out)
	@echo "Opening HTML coverage report..."
	@go tool cover -html=coverage.out

fmt: ## Format Go code
	@echo "Formatting Go code..."
	@go fmt ./...

mocks: ## Regenerate moq mocks (uses `tool github.com/matryer/moq` from go.mod)
	@echo "Running go generate for moq stubs..."
	@go generate ./...

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	@golangci-lint run || echo "golangci-lint not installed, skipping"

tidy: ## Tidy Go module dependencies
	@echo "Tidying Go module dependencies..."
	@go mod tidy

deps: ## Download Go module dependencies
	@echo "Downloading dependencies..."
	@go mod download

# example: make release VERSION=v0.2.0
release: ## Create a release tag and push it (requires VERSION=vX.Y.Z)
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "Error: VERSION must be set (e.g., VERSION=v0.2.0)"; \
		exit 1; \
	fi
	@if ! echo "$(VERSION)" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "Error: VERSION must be in semantic version format (e.g., v0.2.0)"; \
		exit 1; \
	fi
	@echo "Checking git status..."
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: Working directory is not clean. Commit or stash changes first."; \
		exit 1; \
	fi
	@echo "Checking if tag $(VERSION) already exists..."
	@if git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "Error: Tag $(VERSION) already exists"; \
		exit 1; \
	fi
	@echo "Checking if we're on main branch..."
	@if [ "$$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then \
		echo "Warning: Not on main branch. Continuing anyway..."; \
	fi
	@echo "Creating release tag $(VERSION)..."
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "Pushing tag $(VERSION) to remote..."
	@git push origin $(VERSION)
	@echo "Release $(VERSION) created and pushed successfully!"
	@echo "GitHub Actions will automatically create a release with the binary."

# Default target when running `make`
.DEFAULT_GOAL := help
