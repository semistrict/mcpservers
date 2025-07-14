package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"os/exec"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*NewSessionTool]())
}

type NewSessionTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_new_session" title:"Create Tmux Session" description:"Create a new tmux session with optional command execution" destructive:"false"`
	SessionTool
	Command       []string `json:"command" description:"Command and arguments to run in the session"`
	Expect        string   `json:"expect" description:"Wait for this string to appear in output before returning"`
	KillOthers    bool     `json:"kill_others" description:"Kill existing sessions with same prefix before creating new one"`
	AllowMultiple bool     `json:"allow_multiple" description:"Allow multiple sessions with same prefix"`
	MaxWait       float64  `json:"max_wait" description:"Maximum seconds to wait for output"`
}

func (t *NewSessionTool) Handle(ctx context.Context) (interface{}, error) {
	maxWait := t.MaxWait
	if maxWait == 0 {
		maxWait = 10
	}

	prefix := t.Prefix
	if prefix == "" {
		prefix = detectPrefix()
	}

	if t.KillOthers {
		sessions, err := findSessionsByPrefix(prefix)
		if err == nil {
			for _, session := range sessions {
				killSession(session)
			}
		}
	}

	if !t.AllowMultiple {
		existing, err := findSessionsByPrefix(prefix)
		if err == nil && len(existing) > 0 {
			return nil, fmt.Errorf("session with prefix '%s' already exists: %s. Use --allow-multiple or --kill-others", prefix, existing[0])
		}
	}

	sessionName := generateSessionName(prefix, t.Command)

	var cmd *exec.Cmd
	if len(t.Command) > 0 {
		args := append([]string{"new-session", "-d", "-s", sessionName}, t.Command...)
		cmd = exec.Command("tmux", args...)
	} else {
		cmd = exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	}

	if err := cmd.Run(); err != nil {
		if err := cleanupTmuxTempFiles(); err == nil {
			if retryErr := cmd.Run(); retryErr == nil {
				// Success on retry
			} else {
				return nil, fmt.Errorf("failed to create session even after cleanup: %w", retryErr)
			}
		} else {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
	}

	var output string
	if t.Expect != "" {
		result, err := waitForExpected(sessionName, t.Expect, maxWait)
		if err != nil {
			return nil, fmt.Errorf("error creating session: %v", err)
		}
		output = result.Output
	} else {
		result, err := waitForStability(sessionName, maxWait)
		if err != nil {
			return nil, fmt.Errorf("error creating session: %v", err)
		}
		output = result.Output
	}

	return fmt.Sprintf("Session created: %s\nOutput:\n%s", sessionName, output), nil
}
