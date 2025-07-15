package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*SendControlKeysTool]())
}

type SendControlKeysTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_send_control_keys" title:"Send Control Keys to Tmux Session" description:"Send control sequences and special keys to tmux session with hash verification, waits for output to stabilize and returns it (usually not necessary to capture output again). Supports tmux key syntax including modifiers (C-, M-, S-) and special keys (Enter, F1-F12, Up, Down, etc.)" destructive:"true"`
	SessionTool
	Hash    string  `json:"hash" mcp:"required" description:"Content hash from previous capture (required for safety)"`
	Keys    string  `json:"keys" mcp:"required" description:"Control keys to send. Supports tmux syntax: C- (Ctrl), M- (Alt), S- (Shift), special keys (Enter, F1-F12, Up, Down, etc.). Examples: 'C-c', 'M-x', 'F1', 'Enter', 'Up Down Left Right'"`
	Enter   bool    `json:"enter" description:"Append Enter key after sending keys"`
	Expect  string  `json:"expect" mcp:"required" description:"Wait for this string to appear on the cursor line (where user input goes)"`
	MaxWait float64 `json:"max_wait" description:"Maximum seconds to wait for expected output"`
	Hex     bool    `json:"hex" description:"Use hex mode (-H flag): treat keys as hexadecimal ASCII character codes (space-separated)"`
}

func (t *SendControlKeysTool) Handle(ctx context.Context) (interface{}, error) {
	sessionName, err := resolveSession(ctx, t.Prefix, t.Session)
	if err != nil {
		return nil, fmt.Errorf("error sending control keys: %v", err)
	}

	result, err := sendKeysCommon(ctx, SendKeysOptions{
		SessionName: sessionName,
		Hash:        t.Hash,
		Keys:        t.Keys,
		Enter:       t.Enter,
		Expect:      t.Expect,
		MaxWait:     t.MaxWait,
		Literal:     false, // Don't use literal mode - we want tmux to interpret control sequences
		Hex:         t.Hex,
	})
	if err != nil {
		return nil, err
	}

	if result.Output == "" {
		return fmt.Sprintf("Control keys sent to session: %s", result.SessionName), nil
	}
	return fmt.Sprintf("Control keys sent to session: %s\nNew Hash: %s\n\n%s", result.SessionName, result.Hash, result.Output), nil
}
