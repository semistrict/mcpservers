# MCP Servers

A collection of Model Context Protocol (MCP) servers built in Go.

## Available Servers

### Tmux Server (`servers/tmux/`)

Advanced tmux session management with safety features:

- **Hash-based safety**: Prevents accidental command execution when session state has changed
- **Intelligent waiting**: Sophisticated output detection with configurable timeouts  
- **Auto-detection**: Automatically finds sessions based on git repository context
- **Read-only by default**: Safe attachment mode prevents accidental modifications
- **Output formatting**: Line numbers and empty line compression for better readability

**Tools**: `tmux_new_session`, `tmux_capture`, `tmux_send_keys`, `tmux_send_control_keys`, `tmux_list`, `tmux_kill`, `tmux_attach`, `tmux_bash`

See [servers/tmux/README.md](servers/tmux/README.md) for detailed documentation.


## Development

This project includes a comprehensive Makefile for building and testing:

### Quick Start
```bash
make                    # Build and test everything
make build              # Build all binaries to bin/ directory
make test               # Run all tests with verbose output
make clean              # Remove build artifacts
```

### Development Workflow
```bash
make precommit          # Clean, format, vet, staticcheck, build, and test
make fmt                # Format code with go fmt
make vet                # Run go vet for basic issues
make staticcheck        # Run static analysis
```

### CI/CD
```bash
make ci                 # CI workflow (build, test, lint, fmt-check, vet, staticcheck)
make fmt-check          # Verify code is properly formatted
make test-coverage      # Generate HTML coverage report in out/
make lint               # Run golangci-lint (if installed)
```

### Installation
```bash
make install            # Install binaries to $GOPATH/bin
make clean              # Remove build artifacts
```

## Building

Build a specific server:

```bash
# Tmux server
go build -o tmux-mcp ./servers/tmux/cmd/tmux-mcp

# MCP testing utility
go build -o mcptest ./cmd/mcptest

# MCP wrapper (hot-reload during development)
go build -o mcpwrapper ./cmd/mcpwrapper
```

## Testing with mcptest

The repository includes a powerful testing utility for MCP servers:

```bash
# Build the tools first
make build

# Interactive testing
./bin/mcptest ./bin/tmux-mcp

# Run test file
./bin/mcptest ./bin/tmux-mcp test.txt
```


## Development with Hot-Reload

The `mcpwrapper` utility provides hot-reload functionality during development:

```bash
# Start wrapper that monitors for binary changes
./bin/mcpwrapper ./bin/tmux-mcp

# In another terminal, recompile the server
make build

# The wrapper automatically:
# 1. Detects the binary change
# 2. Restarts the underlying server
# 3. Removes all old tools
# 4. Loads new tools from restarted server
# 5. Sends tool list change notifications to connected clients
```

This allows seamless development where you can modify server code, recompile, and immediately see changes in connected MCP clients without manual restarts.


## Contributing

When adding new MCP servers:

1. Create a new directory under `servers/`
2. Follow the established structure with `cmd/`, `pkg/` subdirectories
3. Include a server-specific README.md
4. Update this main README with the new server details

## Requirements

- Go 1.24.5 or later
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) library for MCP protocol implementation