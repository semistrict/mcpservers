package tmuxmcp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// testSocketPath is used for testing to override the default tmux socket
var testSocketPath string

// Options structs
type captureOptions struct {
	Prefix string
}

type captureResult struct {
	SessionName string
	Output      string
	Hash        string
}

type cursorResult struct {
	SessionName string
	CursorLine  string
	CursorY     int
	CursorX     int
	Output      string
	Hash        string
}

// Constants
const (
	expectWaitTimeout  = 60
	noOutputTimeout    = 20
	checkInterval      = 200 * time.Millisecond
	stabilityThreshold = 500 * time.Millisecond
)

func capture(ctx context.Context, opts captureOptions) (*captureResult, error) {
	sessionName, err := resolveSession(ctx, opts.Prefix, "")
	if err != nil {
		return nil, err
	}

	output, err := runTmuxCommand(ctx, "capture-pane", "-t", sessionName, "-p")
	if err != nil {
		return nil, fmt.Errorf("failed to capture session %s: %w", sessionName, err)
	}

	formatted := formatOutput(output)
	hash := calculateHash(output)

	return &captureResult{
		SessionName: sessionName,
		Output:      formatted,
		Hash:        hash,
	}, nil
}

func captureWithCursor(ctx context.Context, opts captureOptions) (*cursorResult, error) {
	sessionName, err := resolveSession(ctx, opts.Prefix, "")
	if err != nil {
		return nil, err
	}

	// Capture output
	output, err := runTmuxCommand(ctx, "capture-pane", "-t", sessionName, "-p")
	if err != nil {
		return nil, fmt.Errorf("failed to capture session %s: %w", sessionName, err)
	}

	// Get cursor position
	cursorOutput, err := runTmuxCommand(ctx, "display-message", "-t", sessionName, "-p", "#{cursor_y}:#{cursor_x}")
	if err != nil {
		return nil, fmt.Errorf("failed to get cursor position for session %s: %w", sessionName, err)
	}

	// Parse cursor position
	cursorPos := strings.TrimSpace(cursorOutput)
	parts := strings.Split(cursorPos, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor position format: %s", cursorPos)
	}

	cursorY, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid cursor Y position: %s", parts[0])
	}

	cursorX, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid cursor X position: %s", parts[1])
	}

	// Extract the line where cursor is positioned
	lines := strings.Split(output, "\n")
	var cursorLine string
	if cursorY >= 0 && cursorY < len(lines) {
		cursorLine = lines[cursorY]
	}

	formatted := formatOutput(output)
	hash := calculateHash(output)

	return &cursorResult{
		SessionName: sessionName,
		CursorLine:  cursorLine,
		CursorY:     cursorY,
		CursorX:     cursorX,
		Output:      formatted,
		Hash:        hash,
	}, nil
}

func list(ctx context.Context, prefix string) ([]string, error) {
	output, err := runTmuxCommand(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return []string{}, nil
	}

	sessions := strings.Split(strings.TrimSpace(output), "\n")
	if prefix == "" {
		return sessions, nil
	}

	var filtered []string
	for _, session := range sessions {
		if strings.HasPrefix(session, prefix) {
			filtered = append(filtered, session)
		}
	}

	return filtered, nil
}

// Helper functions
func detectPrefix() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "tmux"
	}

	repoPath := strings.TrimSpace(string(output))
	repoName := filepath.Base(repoPath)

	reg := regexp.MustCompile(`[^a-zA-Z0-9-_]`)
	sanitized := reg.ReplaceAllString(repoName, "-")

	return sanitized
}

func resolveSession(ctx context.Context, prefix, session string) (string, error) {
	if session != "" {
		sessions, err := list(ctx, "")
		if err != nil {
			return "", err
		}
		for _, s := range sessions {
			if s == session {
				return session, nil
			}
		}
		return "", fmt.Errorf("session '%s' not found", session)
	}

	if prefix == "" {
		prefix = detectPrefix()
	}

	sessions, err := findSessionsByPrefix(ctx, prefix)
	if err != nil {
		return "", err
	}

	if len(sessions) == 0 {
		return "", fmt.Errorf("no sessions found with prefix '%s'", prefix)
	}

	if len(sessions) > 1 {
		return "", fmt.Errorf("multiple sessions found with prefix '%s': %s. Use specific session name", prefix, strings.Join(sessions, ", "))
	}

	return sessions[0], nil
}

func findSessionsByPrefix(ctx context.Context, prefix string) ([]string, error) {
	sessions, err := list(ctx, "")
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, session := range sessions {
		if strings.HasPrefix(session, prefix) {
			matches = append(matches, session)
		}
	}

	return matches, nil
}

func killSession(ctx context.Context, sessionName string) error {
	_, err := runTmuxCommand(ctx, "kill-session", "-t", sessionName)
	return err
}

func formatOutput(output string) string {
	lines := strings.Split(output, "\n")
	var formatted []string
	var emptyCount int

	for i, line := range lines {
		lineNum := i + 1
		if strings.TrimSpace(line) == "" {
			emptyCount++
			if emptyCount == 1 {
				formatted = append(formatted, fmt.Sprintf("[%d]: ", lineNum))
			}
		} else {
			if emptyCount > 1 {
				formatted = append(formatted, fmt.Sprintf("... %d empty testLines ...", emptyCount))
			}
			emptyCount = 0
			formatted = append(formatted, fmt.Sprintf("[%d]: %s", lineNum, line))
		}
	}

	if emptyCount > 1 {
		formatted = append(formatted, fmt.Sprintf("... %d empty testLines ...", emptyCount))
	}

	return strings.Join(formatted, "\n")
}

func calculateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)[:8]
}

func waitForStability(ctx context.Context, sessionName string) (*captureResult, error) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	var lastOutput string
	var lastChange time.Time = time.Now()

	for {
		select {
		case <-ctx.Done():
			result, _ := capture(ctx, captureOptions{Prefix: sessionName})
			if result != nil {
				return result, nil
			}
			return nil, fmt.Errorf("context cancelled waiting for stability: %w", ctx.Err())

		case <-ticker.C:
			result, err := capture(ctx, captureOptions{Prefix: sessionName})
			if err != nil {
				continue
			}

			if result.Output != lastOutput {
				lastOutput = result.Output
				lastChange = time.Now()
			} else if time.Since(lastChange) >= stabilityThreshold {
				return result, nil
			}
		}
	}
}

func waitForExpected(ctx context.Context, sessionName, expected string) (*captureResult, error) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	var lastOutput string
	var lastChange time.Time = time.Now()

	for {
		select {
		case <-ctx.Done():
			result, _ := capture(ctx, captureOptions{Prefix: sessionName})
			if result != nil {
				return result, fmt.Errorf("context cancelled waiting for '%s' on cursor line: %w", expected, ctx.Err())
			}
			return nil, fmt.Errorf("context cancelled waiting for '%s' on cursor line: %w", expected, ctx.Err())

		case <-ticker.C:
			cursorResult, err := captureWithCursor(ctx, captureOptions{Prefix: sessionName})
			if err != nil {
				continue
			}

			// Check if expected text is found on the cursor line only
			if strings.Contains(cursorResult.CursorLine, expected) {
				// Convert cursorResult to captureResult for return
				return &captureResult{
					SessionName: cursorResult.SessionName,
					Output:      cursorResult.Output,
					Hash:        cursorResult.Hash,
				}, nil
			}

			if cursorResult.Output != lastOutput {
				lastOutput = cursorResult.Output
				lastChange = time.Now()
			} else if time.Since(lastChange) >= time.Duration(noOutputTimeout)*time.Second {
				return &captureResult{
					SessionName: cursorResult.SessionName,
					Output:      cursorResult.Output,
					Hash:        cursorResult.Hash,
				}, fmt.Errorf("no new output for %d seconds while waiting for '%s' on cursor line", noOutputTimeout, expected)
			}
		}
	}
}
