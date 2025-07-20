# Makefile for tmux-mcp project

.PHONY: all build test clean install help lint fmt vet staticcheck precommit ci fmt-check test-coverage

TEST_TIMEOUT=90s
TEST_PARALLEL=32
GOPATH=$(shell go env GOPATH)
GOLANGCI_LINT=$(shell command -v golangci-lint 2>/dev/null || echo ${GOPATH}/bin/golangci-lint)
STATICCHECK=$(shell command -v staticcheck 2>/dev/null || echo ${GOPATH}/bin/staticcheck)

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
	scripts/repo-check.sh
	go test -timeout ${TEST_TIMEOUT} -parallel ${TEST_PARALLEL} -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	mkdir -p out
	go test -timeout ${TEST_TIMEOUT} -v -coverprofile=out/coverage.out ./... || echo "Tests failed"
	go tool cover -html=out/coverage.out -o out/coverage.html
	@echo "Coverage report generated: out/coverage.html"

# Lint the code
lint:
	${GOLANGCI_LINT} run
# Format the code
fmt:
	go fmt ./...

# Check if code is properly formatted (for CI)
fmt-check:
	@if [ -n "$$(go fmt ./...)" ]; then \
		echo "❌ Code is not properly formatted. Run 'make fmt' to fix."; \
		exit 1; \
	fi

# Vet the code
vet:
	go vet ./...

# Run staticcheck
staticcheck:
	${STATICCHECK} ./...

precommit: clean fmt vet staticcheck build test
	@echo "Pre-commit checks passed!"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Clean complete!"

# Install binaries to GOPATH/bin
install: build
	cp bin/tmux-mcp $(GOPATH)/bin/
	cp bin/mcptest $(GOPATH)/bin/
	cp bin/mcpwrapper $(GOPATH)/bin/
	@echo "Install complete!"


# CI workflow
ci: build test-coverage lint fmt-check vet staticcheck
	@echo "CI checks passed!"
