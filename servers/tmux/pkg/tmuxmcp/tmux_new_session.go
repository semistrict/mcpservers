package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/semistrict/mcpservers/servers/tmux/pkg/tmux"
)

func init() {
	r.Register(registerNewSessionTool)
}

func registerNewSessionTool(server *Server) {
	tool := mcp.NewTool("tmux_new_session",
		mcp.WithDescription("Create a new tmux session with optional command execution"),
		mcp.WithArray("command",
			mcp.Description("Command and arguments to run in the session"),
		),
		mcp.WithString("prefix",
			mcp.Description("Session name prefix (auto-detected from git repo if not provided)"),
		),
		mcp.WithString("expect",
			mcp.Description("Wait for this string to appear in output before returning"),
		),
		mcp.WithBoolean("kill_others",
			mcp.Description("Kill existing sessions with same prefix before creating new one"),
		),
		mcp.WithBoolean("allow_multiple",
			mcp.Description("Allow multiple sessions with same prefix"),
		),
		mcp.WithNumber("max_wait",
			mcp.Description("Maximum seconds to wait for output"),
		),
	)
	server.AddTool(tool, server.handleNewSession)
}

func (s *Server) handleNewSession(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()

	var command []string
	if cmd, ok := arguments["command"].([]interface{}); ok {
		for _, v := range cmd {
			if str, ok := v.(string); ok {
				command = append(command, str)
			}
		}
	}

	prefix, _ := arguments["prefix"].(string)
	expect, _ := arguments["expect"].(string)
	killOthers, _ := arguments["kill_others"].(bool)
	allowMultiple, _ := arguments["allow_multiple"].(bool)
	maxWait, _ := arguments["max_wait"].(float64)
	if maxWait == 0 {
		maxWait = 10
	}

	result, err := s.tmuxClient.NewSession(tmux.NewSessionOptions{
		Command:       command,
		Prefix:        prefix,
		Expect:        expect,
		KillOthers:    killOthers,
		AllowMultiple: allowMultiple,
		MaxWait:       maxWait,
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error creating session: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Session created: %s\nOutput:\n%s", result.SessionName, result.Output),
			},
		},
	}, nil
}
