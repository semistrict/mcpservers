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

**Tools**: `tmux_new_session`, `tmux_capture`, `tmux_send_keys`, `tmux_list`, `tmux_kill`, `tmux_attach`

See [servers/tmux/README.md](servers/tmux/README.md) for detailed documentation.

## Building

Build a specific server:

```bash
# Tmux server
go build -o tmux-tmuxmcp ./servers/tmux/cmd/tmux-tmuxmcp

# MCP testing utility
go build -o mcptest ./cmd/mcptest

# MCP wrapper (hot-reload during development)
go build -o mcpwrapper ./cmd/mcpwrapper
```

## Installing with Claude

```bash
# Install tmux server directly
claude tmuxmcp add -s user tmux-tmuxmcp "$(pwd)/tmux-mcp"

# OR install with hot-reload wrapper for development
claude tmuxmcp add -s user tmux-tmuxmcp "$(pwd)/mcpwrapper $(pwd)/tmux-mcp"
```

## Testing with mcptest

The repository includes a powerful testing utility for MCP servers:

```bash
# Interactive testing
./mcptest ./tmux-tmuxmcp

# Run test file
./mcptest ./tmux-tmuxmcp examples/tmux-basic.txt
```

### Test File Format

Create simple text files with tool calls:

```
# Comments start with #
tool_name arg1=value1 arg2=value2

# Examples:
tmux_list
tmux_new_session command=[echo,hello] prefix=test
tmux_capture prefix=test
```

See `examples/` directory for more test files.

## Development with Hot-Reload

The `mcpwrapper` utility provides hot-reload functionality during development:

```bash
# Start wrapper that monitors for binary changes
./mcpwrapper ./tmux-tmuxmcp

# In another terminal, recompile the server
go build -o tmux-tmuxmcp ./servers/tmux/cmd/tmux-tmuxmcp

# The wrapper automatically:
# 1. Detects the binary change
# 2. Restarts the underlying server
# 3. Removes all old tools
# 4. Loads new tools from restarted server
# 5. Sends tool list change notifications to connected clients
```

This allows seamless development where you can modify server code, recompile, and immediately see changes in connected MCP clients without manual restarts.

## Repository Structure

```
cmd/
├── mcptest/            # MCP server testing utility
└── mcpwrapper/         # Hot-reload wrapper for development
servers/
├── tmux/               # Tmux session management server
│   ├── cmd/tmux-mcp/   # Main entry point
│   ├── pkg/mcp/        # MCP server implementation
│   ├── pkg/tmux/       # Tmux client library
│   └── README.md       # Tmux-specific documentation
examples/               # Test files for mcptest
├── tmux-basic.txt      # Basic tmux operations
└── tmux-interactive.txt # Interactive session example
```

## Contributing

When adding new MCP servers:

1. Create a new directory under `servers/`
2. Follow the established structure with `cmd/`, `pkg/` subdirectories
3. Include a server-specific README.md
4. Update this main README with the new server details

## Requirements

- Go 1.24.5 or later
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) library for MCP protocol implementation