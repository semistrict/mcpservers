package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"os/exec"
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

	sessionName, err := resolveSession(t.Prefix, t.Session)
	if err != nil {
		return nil, err
	}

	// Verify current hash by capturing current state
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	captureOutput, err := captureCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to verify session state: failed to capture session %s: %v", sessionName, err)
	}

	currentHash := calculateHash(string(captureOutput))
	if currentHash != t.Hash {
		return nil, fmt.Errorf("session state has changed. Expected hash %s, got %s. Please capture current output first and carefully consider whether you still want to kill this session.", t.Hash, currentHash)
	}

	if err := killSession(sessionName); err != nil {
		return nil, fmt.Errorf("failed to kill session %s: %v", sessionName, err)
	}

	return fmt.Sprintf("Session %s killed successfully.", sessionName), nil
}
