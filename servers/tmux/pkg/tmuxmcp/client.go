package tmuxmcp

import (
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

// buildTmuxCommand creates a tmux command with the test socket if set
func buildTmuxCommand(args ...string) *exec.Cmd {
	if testSocketPath != "" {
		// Prepend socket args
		allArgs := append([]string{"-S", testSocketPath}, args...)
		return exec.Command("tmux", allArgs...)
	}
	return exec.Command("tmux", args...)
}

// Options structs
type newSessionOptions struct {
	Command       []string
	Prefix        string
	Expect        string
	KillOthers    bool
	AllowMultiple bool
	MaxWait       time.Duration
}

type newSessionResult struct {
	SessionName string
	Output      string
	Hash        string
}

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

type killOptions struct {
	Prefix string
}

// Constants
const (
	defaultWaitTimeout = 10
	expectWaitTimeout  = 60
	noOutputTimeout    = 20
	checkInterval      = 200 * time.Millisecond
	stabilityThreshold = 500 * time.Millisecond
)

// Main functions used by Tools
func newSession(opts newSessionOptions) (*newSessionResult, error) {
	if opts.Prefix == "" {
		opts.Prefix = detectPrefix()
	}

	if opts.KillOthers {
		sessions, err := findSessionsByPrefix(opts.Prefix)
		if err == nil {
			for _, session := range sessions {
				killSession(session)
			}
		}
	}

	if !opts.AllowMultiple {
		existing, err := findSessionsByPrefix(opts.Prefix)
		if err == nil && len(existing) > 0 {
			return nil, fmt.Errorf("session with prefix '%s' already exists: %s. Use --allow-multiple or --kill-others", opts.Prefix, existing[0])
		}
	}

	sessionName, err := createUniqueSession(opts.Prefix, opts.Command)
	if err != nil {
		return nil, err
	}

	var output string
	var hash string
	if opts.Expect != "" {
		result, err := waitForExpected(sessionName, opts.Expect, opts.MaxWait)
		if err != nil {
			return nil, err
		}
		output = result.Output
		hash = result.Hash
	} else {
		result, err := waitForStability(sessionName, opts.MaxWait)
		if err != nil {
			return nil, err
		}
		output = result.Output
		hash = result.Hash
	}

	return &newSessionResult{
		SessionName: sessionName,
		Output:      output,
		Hash:        hash,
	}, nil
}

func capture(opts captureOptions) (*captureResult, error) {
	sessionName, err := resolveSession(opts.Prefix, "")
	if err != nil {
		return nil, err
	}

	cmd := buildTmuxCommand("capture-pane", "-t", sessionName, "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to capture session %s: %w", sessionName, err)
	}

	formatted := formatOutput(string(output))
	hash := calculateHash(string(output))

	return &captureResult{
		SessionName: sessionName,
		Output:      formatted,
		Hash:        hash,
	}, nil
}

func captureWithCursor(opts captureOptions) (*cursorResult, error) {
	sessionName, err := resolveSession(opts.Prefix, "")
	if err != nil {
		return nil, err
	}

	// Capture output
	captureCmd := buildTmuxCommand("capture-pane", "-t", sessionName, "-p")
	output, err := captureCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to capture session %s: %w", sessionName, err)
	}

	// Get cursor position
	cursorCmd := buildTmuxCommand("display-message", "-t", sessionName, "-p", "#{cursor_y}:#{cursor_x}")
	cursorOutput, err := cursorCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get cursor position for session %s: %w", sessionName, err)
	}

	// Parse cursor position
	cursorPos := strings.TrimSpace(string(cursorOutput))
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
	lines := strings.Split(string(output), "\n")
	var cursorLine string
	if cursorY >= 0 && cursorY < len(lines) {
		cursorLine = lines[cursorY]
	}

	formatted := formatOutput(string(output))
	hash := calculateHash(string(output))

	return &cursorResult{
		SessionName: sessionName,
		CursorLine:  cursorLine,
		CursorY:     cursorY,
		CursorX:     cursorX,
		Output:      formatted,
		Hash:        hash,
	}, nil
}

func list(prefix string) ([]string, error) {
	cmd := buildTmuxCommand("list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return []string{}, nil
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
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

func kill(opts killOptions) (string, error) {
	sessionName, err := resolveSession(opts.Prefix, "")
	if err != nil {
		return "", err
	}

	if err := killSession(sessionName); err != nil {
		return "", err
	}

	return sessionName, nil
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

func generateSessionName(prefix string, command []string) string {
	var cmdPart string
	if len(command) > 0 {
		cmdBase := filepath.Base(command[0])
		if len(cmdBase) > 10 {
			cmdBase = cmdBase[:10]
		}
		reg := regexp.MustCompile(`[^a-zA-Z0-9]`)
		cmdPart = reg.ReplaceAllString(cmdBase, "")
		if cmdPart == "" {
			cmdPart = "session"
		}
	} else {
		cmdPart = "session"
	}

	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%s-%d", prefix, cmdPart, timestamp%10000)
}

func resolveSession(prefix, session string) (string, error) {
	if session != "" {
		sessions, err := list("")
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

	sessions, err := findSessionsByPrefix(prefix)
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

func findSessionsByPrefix(prefix string) ([]string, error) {
	sessions, err := list("")
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

func killSession(sessionName string) error {
	cmd := buildTmuxCommand("kill-session", "-t", sessionName)
	return cmd.Run()
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
				formatted = append(formatted, fmt.Sprintf("... %d empty lines ...", emptyCount))
			}
			emptyCount = 0
			formatted = append(formatted, fmt.Sprintf("[%d]: %s", lineNum, line))
		}
	}

	if emptyCount > 1 {
		formatted = append(formatted, fmt.Sprintf("... %d empty lines ...", emptyCount))
	}

	return strings.Join(formatted, "\n")
}

func calculateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)[:8]
}

func waitForStability(sessionName string, maxWait time.Duration) (*captureResult, error) {
	if maxWait == 0 {
		maxWait = defaultWaitTimeout
	}

	timeout := time.After(maxWait)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	var lastOutput string
	var lastChange time.Time = time.Now()

	for {
		select {
		case <-timeout:
			result, _ := capture(captureOptions{Prefix: sessionName})
			if result != nil {
				return result, nil
			}
			return nil, fmt.Errorf("timeout waiting for stability after %.1f seconds", float64(maxWait)/float64(time.Second))

		case <-ticker.C:
			result, err := capture(captureOptions{Prefix: sessionName})
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

func waitForExpected(sessionName, expected string, maxWait time.Duration) (*captureResult, error) {
	if maxWait == 0 {
		maxWait = expectWaitTimeout
	}

	timeout := time.After(maxWait)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	var lastOutput string
	var lastChange time.Time = time.Now()

	for {
		select {
		case <-timeout:
			result, _ := capture(captureOptions{Prefix: sessionName})
			if result != nil {
				return result, fmt.Errorf("timeout waiting for '%s' on cursor line after %.1f seconds", expected, float64(maxWait)/float64(time.Second))
			}
			return nil, fmt.Errorf("timeout waiting for '%s' on cursor line after %.1f seconds", expected, float64(maxWait)/float64(time.Second))

		case <-ticker.C:
			cursorResult, err := captureWithCursor(captureOptions{Prefix: sessionName})
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

func parseKeyString(keys string, literal bool) []string {
	if literal {
		return []string{keys}
	}

	return strings.Fields(keys)
}
