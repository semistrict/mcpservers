package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/semistrict/mcpservers/servers/tmux/pkg/tmux"
)

func init() {
	r.Register(registerKillTool)
}

func registerKillTool(server *Server) {
	tool := mcp.NewTool("tmux_kill",
		mcp.WithDescription("Kill tmux session"),
		mcp.WithString("prefix",
			mcp.Description("Session name prefix (auto-detected from git repo if not provided)"),
		),
		mcp.WithString("session",
			mcp.Description("Specific session name (overrides prefix)"),
		),
	)
	server.AddTool(tool, server.handleKill)
}

func (s *Server) handleKill(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()
	prefix, _ := arguments["prefix"].(string)
	session, _ := arguments["session"].(string)

	sessionName, err := s.tmuxClient.Kill(tmux.KillOptions{
		Prefix:  prefix,
		Session: session,
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error killing session: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Session killed: %s", sessionName),
			},
		},
	}, nil
}
