package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/semistrict/mcpservers/servers/tmux/pkg/tmux"
)

func init() {
	r.Register(registerAttachTool)
}

func registerAttachTool(server *Server) {
	tool := mcp.NewTool("tmux_attach",
		mcp.WithDescription("Attach to tmux session"),
		mcp.WithString("prefix",
			mcp.Description("Session name prefix (auto-detected from git repo if not provided)"),
		),
		mcp.WithString("session",
			mcp.Description("Specific session name (overrides prefix)"),
		),
		mcp.WithBoolean("read_write",
			mcp.Description("Allow writing to session (read-only by default)"),
		),
		mcp.WithBoolean("new_window",
			mcp.Description("Open in new iTerm window (macOS only)"),
		),
	)
	server.AddTool(tool, server.handleAttach)
}

func (s *Server) handleAttach(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()
	prefix, _ := arguments["prefix"].(string)
	session, _ := arguments["session"].(string)
	readWrite, _ := arguments["read_write"].(bool)
	newWindow, _ := arguments["new_window"].(bool)

	sessionName, err := s.tmuxClient.Attach(tmux.AttachOptions{
		Prefix:    prefix,
		Session:   session,
		ReadWrite: readWrite,
		NewWindow: newWindow,
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error attaching to session: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	mode := "read-only"
	if readWrite {
		mode = "read-write"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Attached to session: %s (%s mode)", sessionName, mode),
			},
		},
	}, nil
}
