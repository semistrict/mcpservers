package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/semistrict/mcpservers/servers/tmux/pkg/tmux"
)

func init() {
	r.Register(registerSendKeysTool)
}

func registerSendKeysTool(server *Server) {
	tool := mcp.NewTool("tmux_send_keys",
		mcp.WithDescription("Send keys to tmux session with hash verification. Supports full tmux key syntax including modifiers (C-, M-, S-) and special keys (Enter, F1-F12, Up, Down, etc.)"),
		mcp.WithString("hash",
			mcp.Description("Content hash from previous capture (required for safety)"),
			mcp.Required(),
		),
		mcp.WithString("keys",
			mcp.Description("Keys to send. Supports tmux syntax: literals, C- (Ctrl), M- (Alt), S- (Shift), special keys (Enter, F1-F12, Up, Down, etc.). Examples: 'C-c', 'M-x', 'F1', 'hello world'"),
			mcp.Required(),
		),
		mcp.WithString("prefix",
			mcp.Description("Session name prefix (auto-detected from git repo if not provided)"),
		),
		mcp.WithString("session",
			mcp.Description("Specific session name (overrides prefix)"),
		),
		mcp.WithBoolean("enter",
			mcp.Description("Append Enter key after sending keys"),
		),
		mcp.WithString("expect",
			mcp.Description("Wait for this string to appear in output after sending keys"),
		),
		mcp.WithNumber("max_wait",
			mcp.Description("Maximum seconds to wait for expected output"),
		),
		mcp.WithBoolean("literal",
			mcp.Description("Use literal mode (-l flag): treat keys as literal UTF-8 characters with no special interpretation"),
		),
		mcp.WithBoolean("hex",
			mcp.Description("Use hex mode (-H flag): treat keys as hexadecimal ASCII character codes (space-separated)"),
		),
	)
	server.AddTool(tool, server.handleSendKeys)
}

func (s *Server) handleSendKeys(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := req.GetArguments()
	hash, _ := arguments["hash"].(string)
	keys, _ := arguments["keys"].(string)
	prefix, _ := arguments["prefix"].(string)
	session, _ := arguments["session"].(string)
	enter, _ := arguments["enter"].(bool)
	expect, _ := arguments["expect"].(string)
	maxWait, _ := arguments["max_wait"].(float64)
	literal, _ := arguments["literal"].(bool)
	hex, _ := arguments["hex"].(bool)
	if maxWait == 0 {
		maxWait = 10
	}

	result, err := s.tmuxClient.SendKeys(tmux.SendKeysOptions{
		Hash:    hash,
		Keys:    keys,
		Prefix:  prefix,
		Session: session,
		Enter:   enter,
		Expect:  expect,
		MaxWait: maxWait,
		Literal: literal,
		Hex:     hex,
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error sending keys: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Keys sent to session: %s\nNew Hash: %s\n\n%s", result.SessionName, result.Hash, result.Output),
			},
		},
	}, nil
}
