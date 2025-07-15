package tmuxmcp

import (
	"fmt"
	"time"
)

// SendKeysOptions contains all options for sending keys to a tmux session
type SendKeysOptions struct {
	SessionName string
	Hash        string
	Keys        string
	Enter       bool
	Expect      string
	MaxWait     float64
	Literal     bool // Use literal mode (-l flag)
	Hex         bool // Use hex mode (-H flag)
}

// SendKeysResult contains the result of sending keys to a tmux session
type SendKeysResult struct {
	SessionName string
	Output      string
	Hash        string
}

// sendKeysToSession handles the actual tmux send-keys command execution
func sendKeysToSession(opts SendKeysOptions) error {
	// Build tmux send-keys command
	args := []string{"send-keys", "-t", opts.SessionName}

	if opts.Hex {
		args = append(args, "-H")
	} else if opts.Literal {
		args = append(args, "-l")
	}
	// Note: No flags means tmux interprets control sequences

	args = append(args, opts.Keys)

	// Execute the command
	cmd := buildTmuxCommand(args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send keys to session %s: %w", opts.SessionName, err)
	}

	// Send Enter in a separate command if needed
	if opts.Enter {
		enterCmd := buildTmuxCommand("send-keys", "-t", opts.SessionName, "Enter")
		if err := enterCmd.Run(); err != nil {
			return fmt.Errorf("failed to send Enter key to session %s: %w", opts.SessionName, err)
		}
	}

	return nil
}

// sendKeysCommon is the shared implementation for sending keys to a tmux session
func sendKeysCommon(opts SendKeysOptions) (*SendKeysResult, error) {
	if opts.Hash == "" {
		return nil, fmt.Errorf("hash is required for safety. Please capture the session first with tmux_capture to get the current hash, then use that hash in the send keys tool")
	}

	if opts.Keys == "" {
		return nil, fmt.Errorf("keys parameter is required. Specify the keys to send to the session")
	}

	if opts.MaxWait == 0 {
		opts.MaxWait = 10
	}

	// For non-empty expect, do hash verification
	captureCmd := buildTmuxCommand("capture-pane", "-t", opts.SessionName, "-p")
	captureOutput, err := captureCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to verify session state: failed to capture session %s: %v", opts.SessionName, err)
	}

	currentHash := calculateHash(string(captureOutput))
	if currentHash != opts.Hash {
		return nil, fmt.Errorf("session state has changed. Please capture current output first and carefully consider whether the sent keys still make sense")
	}

	// Send keys to session
	err = sendKeysToSession(opts)
	if err != nil {
		return nil, err
	}

	// Handle output based on expect parameter
	if opts.Expect != "" {
		// Wait for expected text on cursor line and return output
		result, err := waitForExpected(opts.SessionName, opts.Expect, time.Duration(opts.MaxWait)*time.Second)
		if err != nil {
			return nil, fmt.Errorf("error sending keys: %v", err)
		}
		return &SendKeysResult{
			SessionName: opts.SessionName,
			Output:      result.Output,
			Hash:        result.Hash,
		}, nil
	} else {
		// No expect parameter - just send keys and return without waiting or output
		return &SendKeysResult{
			SessionName: opts.SessionName,
			Output:      "",
			Hash:        "",
		}, nil
	}
}
