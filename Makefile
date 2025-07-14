# Makefile for tmux-mcp project

.PHONY: all build test clean install help lint fmt vet

# Default target
all: build test

# Build all binaries
build:
	@echo "Building tmux-mcp..."
	go build -o bin/tmux-mcp ./servers/tmux/cmd/tmux-mcp
	@echo "Building mcptest..."
	go build -o bin/mcptest ./cmd/mcptest
	@echo "Building mcpwrapper..."
	go build -o bin/mcpwrapper ./cmd/mcpwrapper
	@echo "Build complete!"

# Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run only tmux tests
test-tmux:
	@echo "Running tmux tests..."
	go test -v ./servers/tmux/pkg/tmuxmcp

# Run tmux tests with short timeout for CI
test-tmux-short:
	@echo "Running tmux tests (short)..."
	go test -v -timeout=30s ./servers/tmux/pkg/tmuxmcp

# Lint the code
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Format the code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Check if code is properly formatted (for CI)
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$(go fmt ./...)" ]; then \
		echo "❌ Code is not properly formatted. Run 'make fmt' to fix."; \
		exit 1; \
	else \
		echo "✅ Code is properly formatted."; \
	fi

# Vet the code
vet:
	@echo "Vetting code..."
	go vet ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Clean complete!"

# Install binaries to GOPATH/bin
install: build
	@echo "Installing binaries..."
	cp bin/tmux-mcp $(GOPATH)/bin/
	cp bin/mcptest $(GOPATH)/bin/
	cp bin/mcpwrapper $(GOPATH)/bin/
	@echo "Install complete!"

# Show help for tmux-mcp
help-tmux:
	@echo "Showing tmux-mcp help..."
	./bin/tmux-mcp -h

# Run quick checks (format, vet, build, test)
check: fmt vet build test

# Development workflow
dev: clean check

# CI workflow
ci: fmt-check vet build test-tmux-short

# Show available targets
help:
	@echo "Available targets:"
	@echo "  all           - Build and test (default)"
	@echo "  build         - Build all binaries"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-tmux     - Run only tmux tests"
	@echo "  test-tmux-short - Run tmux tests with short timeout"
	@echo "  lint          - Run linter (requires golangci-lint)"
	@echo "  fmt           - Format code"
	@echo "  fmt-check     - Check if code is properly formatted (for CI)"
	@echo "  vet           - Vet code"
	@echo "  clean         - Remove build artifacts"
	@echo "  install       - Install binaries to GOPATH/bin"
	@echo "  help-tmux     - Show tmux-mcp help"
	@echo "  check         - Run fmt, vet, build, test"
	@echo "  dev           - Clean and run checks (development workflow)"
	@echo "  ci            - CI workflow (fmt-check, vet, build, test-short)"
	@echo "  help          - Show this help"