package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"strings"
	"time"
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
	sessionName, err := resolveSession(ctx, t.Prefix, t.Session)
	if err != nil {
		return nil, fmt.Errorf("error sending keys: %v", err)
	}

	// If expect is provided, use the common function
	if t.Expect != "" {
		return t.handleWithExpected(ctx, sessionName)
	}

	// Custom behavior for send_keys without expect
	return t.handleWithoutExpect(ctx, sessionName)
}

func (t *SendKeysTool) handleWithExpected(ctx context.Context, sessionName string) (interface{}, error) {
	result, err := sendKeysCommon(ctx, SendKeysOptions{
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

func (t *SendKeysTool) handleWithoutExpect(ctx context.Context, sessionName string) (interface{}, error) {
	if err := t.validateInput(); err != nil {
		return nil, err
	}

	if err := verifySessionHash(ctx, sessionName, t.Hash); err != nil {
		return nil, err
	}

	if err := sendKeysToSession(ctx, SendKeysOptions{
		SessionName: sessionName,
		Keys:        t.Keys,
		Enter:       false, // We'll handle Enter separately
		Literal:     true,
	}); err != nil {
		return nil, err
	}

	if err := t.waitForKeysToAppear(ctx, sessionName); err != nil {
		return nil, err
	}

	if t.Enter {
		_, err := runTmuxCommand(ctx, "send-keys", "-t", sessionName, "Enter")
		if err != nil {
			return nil, fmt.Errorf("failed to send Enter key to session %s: %w", sessionName, err)
		}
	}

	maxWait := t.MaxWait
	if maxWait == 0 {
		maxWait = 10
	}

	// Create context with deadline for stability wait
	ctxWithTimeout, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(maxWait)*time.Second))
	defer cancel()
	
	stableResult, err := waitForStability(ctxWithTimeout, sessionName)
	if err != nil {
		return nil, fmt.Errorf("error waiting for stability: %v", err)
	}

	return fmt.Sprintf("Keys sent to session: %s\nNew Hash: %s\n\n%s", sessionName, stableResult.Hash, stableResult.Output), nil
}

func (t *SendKeysTool) validateInput() error {
	if t.Hash == "" {
		return fmt.Errorf("hash is required for safety. Please capture the session first with tmux_capture to get the current hash, then use that hash in the send keys tool")
	}

	if t.Keys == "" {
		return fmt.Errorf("keys parameter is required. Specify the keys to send to the session")
	}

	return nil
}

func (t *SendKeysTool) waitForKeysToAppear(ctx context.Context, sessionName string) error {
	maxWait := t.MaxWait
	if maxWait == 0 {
		maxWait = 10
	}

	ctxWithTimeout, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(maxWait)*time.Second))
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctxWithTimeout.Done():
			return fmt.Errorf("timeout waiting for keys '%s' to appear after %.1f seconds", t.Keys, maxWait)

		case <-ticker.C:
			result, err := capture(ctx, captureOptions{Prefix: sessionName})
			if err != nil {
				continue
			}

			if strings.Contains(result.Output, t.Keys) {
				return nil
			}
		}
	}
}
