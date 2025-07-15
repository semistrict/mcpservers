package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*SendKeysTool]())
}

type SendKeysTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_send_keys" title:"Send Text to Tmux Session" description:"Send literal text to tmux session with hash verification, waits for output to stabilize and returns it (usually not necessary to capture output again). Text is sent exactly as provided, preserving spaces and special characters." destructive:"true"`
	SessionTool
	Hash    string  `json:"hash,required" description:"Content hash from previous capture (required for safety)"`
	Keys    string  `json:"keys,required" description:"Text to send to the session. Will be sent exactly as provided, preserving spaces and special characters."`
	Enter   bool    `json:"enter" description:"Append Enter key after sending keys"`
	Expect  string  `json:"expect,required" description:"Wait for this string to appear on the cursor line (where user input goes)"`
	MaxWait float64 `json:"max_wait" description:"Maximum seconds to wait for expected output"`
}

func (t *SendKeysTool) Handle(ctx context.Context) (interface{}, error) {
	sessionName, err := resolveSession(t.Prefix, t.Session)
	if err != nil {
		return nil, fmt.Errorf("error sending keys: %v", err)
	}

	result, err := sendKeysCommon(SendKeysOptions{
		SessionName: sessionName,
		Hash:        t.Hash,
		Keys:        t.Keys,
		Enter:       t.Enter,
		Expect:      t.Expect,
		MaxWait:     t.MaxWait,
		Literal:     true,
	})
	if err != nil {
		return nil, err
	}

	if result.Output == "" {
		return fmt.Sprintf("Keys sent to session: %s", result.SessionName), nil
	}
	return fmt.Sprintf("Keys sent to session: %s\nNew Hash: %s\n\n%s", result.SessionName, result.Hash, result.Output), nil
}
