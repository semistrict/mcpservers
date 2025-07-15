package tmuxmcp

import (
	"context"
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
func sendKeysToSession(ctx context.Context, opts SendKeysOptions) error {
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
	_, err := runTmuxCommand(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to send keys to session %s: %w", opts.SessionName, err)
	}

	// Send Enter in a separate command if needed
	if opts.Enter {
		_, err := runTmuxCommand(ctx, "send-keys", "-t", opts.SessionName, "Enter")
		if err != nil {
			return fmt.Errorf("failed to send Enter key to session %s: %w", opts.SessionName, err)
		}
	}

	return nil
}

// sendKeysCommon is the shared implementation for sending keys to a tmux session
func sendKeysCommon(ctx context.Context, opts SendKeysOptions) (*SendKeysResult, error) {
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
	if err := verifySessionHash(ctx, opts.SessionName, opts.Hash); err != nil {
		return nil, err
	}

	// Send keys to session
	if err := sendKeysToSession(ctx, opts); err != nil {
		return nil, err
	}

	// Handle output based on expect parameter
	if opts.Expect != "" {
		// Wait for expected text on cursor line and return output
		maxWait := opts.MaxWait
		if maxWait == 0 {
			maxWait = expectWaitTimeout
		}
		ctxWithTimeout, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(maxWait)*time.Second))
		defer cancel()
		result, err := waitForExpected(ctxWithTimeout, opts.SessionName, opts.Expect)
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

// verifySessionHash verifies the current session state matches the expected hash
func verifySessionHash(ctx context.Context, sessionName, expectedHash string) error {
	captureOutput, err := runTmuxCommand(ctx, "capture-pane", "-t", sessionName, "-p")
	if err != nil {
		return fmt.Errorf("failed to verify session state: failed to capture session %s: %v", sessionName, err)
	}

	currentHash := calculateHash(captureOutput)
	if currentHash != expectedHash {
		return fmt.Errorf("session state has changed. Please capture current output first and carefully consider whether the sent keys still make sense")
	}

	return nil
}
