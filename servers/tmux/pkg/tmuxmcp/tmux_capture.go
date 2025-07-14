package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/semistrict/mcpservers/servers/tmux/pkg/tmux"
)

func init() {
	r.Register(registerCaptureTool)
}

func registerCaptureTool(server *Server) {
	tool := mcp.NewTool("tmux_capture",
		mcp.WithDescription("Capture output from tmux session with content hash"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("prefix",
			mcp.Description("Session name prefix (auto-detected from git repo if not provided)"),
		),
		mcp.WithString("session",
			mcp.Description("Specific session name (overrides prefix)"),
		),
	)
	server.AddTool(tool, server.handleCapture)
}

func (s *Server) handleCapture(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()
	prefix, _ := arguments["prefix"].(string)
	session, _ := arguments["session"].(string)

	result, err := s.tmuxClient.Capture(tmux.CaptureOptions{
		Prefix:  prefix,
		Session: session,
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error capturing session: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Session: %s\nHash: %s\n\n%s", result.SessionName, result.Hash, result.Output),
			},
		},
	}, nil
}
