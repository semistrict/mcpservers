package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"time"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*NewSessionTool]())
}

type NewSessionTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_new_session" title:"Create Tmux Session" description:"Create a new tmux session with optional command execution" destructive:"true"`
	SessionTool
	Command        []string `json:"command" description:"Command and arguments to run in the session"`
	Expect         string   `json:"expect" description:"Wait for this string to appear in output before returning"`
	KillOthers     bool     `json:"kill_others" description:"Kill existing sessions with same prefix before creating new one"`
	AllowMultiple  bool     `json:"allow_multiple" description:"Allow multiple sessions with same prefix"`
	MaxWait        float64  `json:"max_wait" description:"Maximum seconds to wait for output"`
	OpenInTerminal bool     `json:"open_in_terminal" description:"Also open a view into the session (in read-only mode) in the user's terminal" default:"true"`
}

func (t *NewSessionTool) Handle(ctx context.Context) (interface{}, error) {
	maxWait := time.Duration(t.MaxWait) * time.Second
	if maxWait == 0 {
		maxWait = 10
	}

	prefix := t.Prefix
	if prefix == "" {
		prefix = detectPrefix()
	}

	if t.KillOthers {
		sessions, err := findSessionsByPrefix(ctx, prefix)
		if err == nil {
			for _, session := range sessions {
				killSession(ctx, session)
			}
		}
	}

	if !t.AllowMultiple {
		existing, err := findSessionsByPrefix(ctx, prefix)
		if err == nil && len(existing) > 0 {
			return nil, fmt.Errorf("session with prefix '%s' already exists: %s. Use --allow-multiple or --kill-others", prefix, existing[0])
		}
	}

	sessionName, err := createUniqueSession(ctx, prefix, t.Command)
	if err != nil {
		return nil, err
	}

	var output string
	if t.Expect != "" {
		ctxWithTimeout := ctx
		if maxWait > 0 {
			var cancel context.CancelFunc
			ctxWithTimeout, cancel = context.WithDeadline(ctx, time.Now().Add(maxWait))
			defer cancel()
		}
		result, err := waitForExpected(ctxWithTimeout, sessionName, t.Expect)
		if err != nil {
			return nil, fmt.Errorf("error creating session: %v", err)
		}
		output = result.Output
	} else {
		// Create context with timeout for stability wait
		ctxWithTimeout := ctx
		if maxWait > 0 {
			var cancel context.CancelFunc
			ctxWithTimeout, cancel = context.WithDeadline(ctx, time.Now().Add(maxWait))
			defer cancel()
		}
		result, err := waitForStability(ctxWithTimeout, sessionName)
		if err != nil {
			return nil, fmt.Errorf("error creating session: %v", err)
		}
		output = result.Output
	}

	// Open in terminal if requested (default is true)
	if t.OpenInTerminal {
		if err := openSessionInTerminal(sessionName); err != nil {
			// Don't fail the entire operation if terminal opening fails
			return fmt.Sprintf("Session created: %s\nOutput:\n%s\n\nNote: Could not open in terminal: %v", sessionName, output, err), nil
		}
		return fmt.Sprintf("Session created: %s\nOpened in terminal in read-only mode\nOutput:\n%s", sessionName, output), nil
	}

	return fmt.Sprintf("Session created: %s\nOutput:\n%s", sessionName, output), nil
}
