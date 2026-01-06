.PHONY: build install clean test fmt lint help release

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

clean: ## Remove the built binary
	@echo "Cleaning build artifacts..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Clean complete"

test: ## Run tests
	@echo "Running tests..."
	@go test ./...

fmt: ## Format Go code
	@echo "Formatting Go code..."
	@go fmt ./...

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
