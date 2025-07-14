package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
)

func init() {
	r.Register(registerListTool)
}

func registerListTool(server *Server) {
	tool := mcp.NewTool("tmux_list",
		mcp.WithDescription("List all tmux sessions"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("prefix",
			mcp.Description("Filter sessions by prefix"),
		),
	)
	server.AddTool(tool, server.handleList)
}

func (s *Server) handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()
	prefix, _ := arguments["prefix"].(string)

	sessions, err := s.tmuxClient.List(prefix)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error listing sessions: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	var output string
	if len(sessions) == 0 {
		output = "No tmux sessions found"
		if prefix != "" {
			output += fmt.Sprintf(" with prefix '%s'", prefix)
		}
	} else {
		output = "Tmux sessions:\n"
		for _, session := range sessions {
			output += fmt.Sprintf("- %s\n", session)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: output,
			},
		},
	}, nil
}
