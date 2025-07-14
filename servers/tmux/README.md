# Tmux MCP Server

A Model Context Protocol (MCP) server for advanced tmux session management.

## Features

- **Hash-based safety**: Prevents accidental command execution when session state has changed
- **Intelligent waiting**: Sophisticated output detection with configurable timeouts  
- **Auto-detection**: Automatically finds sessions based on git repository context
- **Read-only by default**: Safe attachment mode prevents accidental modifications
- **Output formatting**: Line numbers and empty line compression for better readability

## Building

From the repository root:

```bash
go build -o tmux-tmuxmcp ./servers/tmux/cmd/tmux-tmuxmcp
```

## Installing with Claude

```bash
claude tmuxmcp add -s user tmux-tmuxmcp "$(pwd)/tmux-mcp"
```

## Tools

- `tmux_new_session` - Create a new tmux session with optional command execution
- `tmux_capture` - Capture output from tmux session with content hash (read-only)
- `tmux_send_keys` - Send keys to tmux session with hash verification
- `tmux_list` - List all tmux sessions (read-only)
- `tmux_kill` - Kill tmux session
- `tmux_attach` - Attach to tmux session

## Safety Features

The hash-based safety system ensures commands are only executed if the session state matches expectations. When capturing output, the tool generates a SHA256 hash (first 8 characters) of the current session content. This hash must be provided when sending keys, ensuring commands are only executed if the session state hasn't changed.

## Configuration

Sessions are automatically detected based on the current git repository name. The server sanitizes repo names for tmux compatibility and falls back to 'tmux' prefix if not in a git repo.