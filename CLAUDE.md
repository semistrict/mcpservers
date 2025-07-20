# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

- `make build` - Build all binaries to `bin/` directory (required after code changes)
- `make precommit` - Pre-commit checks: clean, format, vet, staticcheck, build, and test
- `make clean` - Remove build artifacts

## Test Commands

- `make test` - Run all tests with verbose output
- `make test-coverage` - Generate HTML coverage report
- Run a single test: `go test -v -run TestName ./servers/tmux/pkg/tmuxmcp`
- Run tests in a specific package: `go test -v ./servers/tmux/pkg/tmuxmcp`

## Code Quality

- `make fmt` - Format code with go fmt
- `make vet` - Run go vet for basic issues
- `make staticcheck` - Run static analysis
- `make lint` - Run golangci-lint (if installed)
- `make ci` - Run CI workflow (build, test, lint, fmt-check, vet, staticcheck)

## Architecture Overview

This is a collection of Model Context Protocol (MCP) servers in Go with two main components:

### Core Structure
- **servers/tmux/** - Tmux session management with hash-based safety
- **pkg/mcpcommon/** - Shared utilities and base types
- **cmd/** - Development tools (mcptest, mcpwrapper)

### Key Design Patterns
1. **Tool Registration**: Each MCP tool is a struct implementing a Handle method, registered with the MCP server
2. **Hash-Based Safety**: Tmux operations use content hashes to prevent executing commands when session state has changed
3. **Context Timeouts**: All operations use context deadlines instead of raw timeout parameters
4. **Session Auto-Detection**: Sessions are prefixed based on git repository name for isolation

### Testing Approach
- Standard Go testing with `testing.T`
- Integration tests for tmux operations with real tmux sessions
- Test utilities in `mcptest` for manual testing

## Development Guidelines

- Do not cd into subdirectories - always work from repository root
- Always run `make build` after code changes
- Use context.WithDeadline for timeouts, not raw time parameters
- Follow existing patterns for tool implementation (see existing tools in servers/*/pkg/)
- Test files should avoid magic strings and brittle assumptions