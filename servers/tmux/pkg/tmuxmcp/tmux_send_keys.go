package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"os/exec"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*SendKeysTool]())
}

type SendKeysTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_send_keys" title:"Send Keys to Tmux Session" description:"Send keys to tmux session with hash verification. Supports full tmux key syntax including modifiers (C-, M-, S-) and special keys (Enter, F1-F12, Up, Down, etc.)" destructive:"false"`
	SessionTool
	Hash    string  `json:"hash,required" description:"Content hash from previous capture (required for safety)"`
	Keys    string  `json:"keys,required" description:"Keys to send. Supports tmux syntax: literals, C- (Ctrl), M- (Alt), S- (Shift), special keys (Enter, F1-F12, Up, Down, etc.). Examples: 'C-c', 'M-x', 'F1', 'hello world'"`
	Enter   bool    `json:"enter" description:"Append Enter key after sending keys"`
	Expect  string  `json:"expect" description:"Wait for this string to appear in output after sending keys"`
	MaxWait float64 `json:"max_wait" description:"Maximum seconds to wait for expected output"`
	Literal bool    `json:"literal" description:"Use literal mode (-l flag): treat keys as literal UTF-8 characters with no special interpretation"`
	Hex     bool    `json:"hex" description:"Use hex mode (-H flag): treat keys as hexadecimal ASCII character codes (space-separated)"`
}

func (t *SendKeysTool) Handle(ctx context.Context) (interface{}, error) {
	maxWait := t.MaxWait
	if maxWait == 0 {
		maxWait = 10
	}

	if t.Hash == "" {
		return nil, fmt.Errorf("hash is required for safety. Please capture the session first with tmux_capture to get the current hash, then use that hash in tmux_send_keys")
	}

	if t.Keys == "" {
		return nil, fmt.Errorf("keys parameter is required. Specify the keys to send to the session")
	}

	sessionName, err := resolveSession(t.Prefix, t.Session)
	if err != nil {
		return nil, fmt.Errorf("error sending keys: %v", err)
	}

	// Verify current hash by capturing current state
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	captureOutput, err := captureCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to verify session state: failed to capture session %s: %v", sessionName, err)
	}

	currentHash := calculateHash(string(captureOutput))
	if currentHash != t.Hash {
		return nil, fmt.Errorf("session state has changed. Expected hash %s, got %s. Please capture current output first and carefully consider whether the sent keys still make sense.", t.Hash, currentHash)
	}

	args := []string{"send-keys", "-t", sessionName}

	if t.Literal {
		args = append(args, "-l")
	}
	if t.Hex {
		args = append(args, "-H")
	}

	if t.Keys != "" {
		if err := validateKeyString(t.Keys, t.Literal, t.Hex); err != nil {
			return nil, fmt.Errorf("invalid key string: %w", err)
		}

		keyParts := parseKeyString(t.Keys, t.Literal)
		args = append(args, keyParts...)
	}

	if t.Enter {
		args = append(args, "Enter")
	}

	cmd := exec.Command("tmux", args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to send keys to session %s: %w", sessionName, err)
	}

	var output string
	var hash string
	if t.Expect != "" {
		result, err := waitForExpected(sessionName, t.Expect, maxWait)
		if err != nil {
			return nil, fmt.Errorf("error sending keys: %v", err)
		}
		output = result.Output
		hash = result.Hash
	} else {
		result, err := waitForStability(sessionName, maxWait)
		if err != nil {
			return nil, fmt.Errorf("error sending keys: %v", err)
		}
		output = result.Output
		hash = result.Hash
	}

	return fmt.Sprintf("Keys sent to session: %s\nNew Hash: %s\n\n%s", sessionName, hash, output), nil
}
