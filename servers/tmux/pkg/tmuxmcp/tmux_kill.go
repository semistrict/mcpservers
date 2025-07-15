package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*KillTool]())
}

type KillTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_kill" title:"Kill Tmux Session" description:"Kill a tmux session" destructive:"true"`
	SessionTool
	Hash string `json:"hash,required" description:"Content hash from previous capture (required for safety)"`
}

func (t *KillTool) Handle(ctx context.Context) (any, error) {
	if t.Hash == "" {
		return nil, fmt.Errorf("hash is required for safety. Please capture the session first with tmux_capture to get the current hash, then use that hash in tmux_kill")
	}

	sessionName, err := resolveSession(ctx, t.Prefix, t.Session)
	if err != nil {
		return nil, err
	}

	// Verify current hash by capturing current state
	if err := verifySessionHash(ctx, sessionName, t.Hash); err != nil {
		return nil, err
	}

	if err := killSession(ctx, sessionName); err != nil {
		return nil, fmt.Errorf("failed to kill session %s: %v", sessionName, err)
	}

	return fmt.Sprintf("Session %s killed successfully.", sessionName), nil
}
