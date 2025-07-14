package tmuxmcp

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Options structs
type newSessionOptions struct {
	Command       []string
	Prefix        string
	Expect        string
	KillOthers    bool
	AllowMultiple bool
	MaxWait       float64
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

type sendKeysOptions struct {
	Hash    string
	Keys    string
	Prefix  string
	Enter   bool
	Expect  string
	MaxWait float64
	Literal bool
	Hex     bool
}

type sendKeysResult struct {
	SessionName string
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

	sessionName := generateSessionName(opts.Prefix, opts.Command)

	var cmd *exec.Cmd
	if len(opts.Command) > 0 {
		args := append([]string{"new-session", "-d", "-s", sessionName}, opts.Command...)
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

	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
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

func sendKeys(opts sendKeysOptions) (*sendKeysResult, error) {
	if opts.Hash == "" {
		return nil, fmt.Errorf("hash is required for safety. Please capture the session first with tmux_capture to get the current hash, then use that hash in tmux_send_keys")
	}

	if opts.Keys == "" {
		return nil, fmt.Errorf("keys parameter is required. Specify the keys to send to the session")
	}

	sessionName, err := resolveSession(opts.Prefix, "")
	if err != nil {
		return nil, err
	}

	current, err := capture(captureOptions{Prefix: opts.Prefix})
	if err != nil {
		return nil, fmt.Errorf("failed to verify session state: %w", err)
	}

	if current.Hash != opts.Hash {
		return nil, fmt.Errorf("session state has changed. Expected hash %s, got %s. Please capture current output first and carefully consider whether the sent keys still make sense.", opts.Hash, current.Hash)
	}

	args := []string{"send-keys", "-t", sessionName}

	if opts.Literal {
		args = append(args, "-l")
	}
	if opts.Hex {
		args = append(args, "-H")
	}

	if opts.Keys != "" {
		if err := validateKeyString(opts.Keys, opts.Literal, opts.Hex); err != nil {
			return nil, fmt.Errorf("invalid key string: %w", err)
		}

		keyParts := parseKeyString(opts.Keys, opts.Literal)
		args = append(args, keyParts...)
	}

	if opts.Enter {
		args = append(args, "Enter")
	}

	cmd := exec.Command("tmux", args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to send keys to session %s: %w", sessionName, err)
	}

	var result *captureResult
	if opts.Expect != "" {
		result, err = waitForExpected(sessionName, opts.Expect, opts.MaxWait)
	} else {
		result, err = waitForStability(sessionName, opts.MaxWait)
	}
	if err != nil {
		return nil, err
	}

	return &sendKeysResult{
		SessionName: sessionName,
		Output:      result.Output,
		Hash:        result.Hash,
	}, nil
}

func list(prefix string) ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
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
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
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
	if strings.TrimSpace(content) == "" {
		return "empty"
	}

	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)[:8]
}

func waitForStability(sessionName string, maxWait float64) (*captureResult, error) {
	if maxWait == 0 {
		maxWait = defaultWaitTimeout
	}

	timeout := time.After(time.Duration(maxWait) * time.Second)
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
			return nil, fmt.Errorf("timeout waiting for stability after %.1f seconds", maxWait)

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

func waitForExpected(sessionName, expected string, maxWait float64) (*captureResult, error) {
	if maxWait == 0 {
		maxWait = expectWaitTimeout
	}

	timeout := time.After(time.Duration(maxWait) * time.Second)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	var lastOutput string
	var lastChange time.Time = time.Now()

	for {
		select {
		case <-timeout:
			result, _ := capture(captureOptions{Prefix: sessionName})
			if result != nil {
				return result, fmt.Errorf("timeout waiting for '%s' after %.1f seconds", expected, maxWait)
			}
			return nil, fmt.Errorf("timeout waiting for '%s' after %.1f seconds", expected, maxWait)

		case <-ticker.C:
			result, err := capture(captureOptions{Prefix: sessionName})
			if err != nil {
				continue
			}

			if strings.Contains(result.Output, expected) {
				return result, nil
			}

			if result.Output != lastOutput {
				lastOutput = result.Output
				lastChange = time.Now()
			} else if time.Since(lastChange) >= time.Duration(noOutputTimeout)*time.Second {
				return result, fmt.Errorf("no new output for %d seconds while waiting for '%s'", noOutputTimeout, expected)
			}
		}
	}
}

func cleanupTmuxTempFiles() error {
	tmpDir := "/tmp"
	pattern := filepath.Join(tmpDir, "tmux-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find tmux temp files: %w", err)
	}

	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil {
			continue
		}
	}

	return nil
}

func validateKeyString(keys string, literal, hex bool) error {
	if keys == "" {
		return fmt.Errorf("keys cannot be empty")
	}

	if literal || hex {
		if hex {
			parts := strings.Fields(keys)
			for _, part := range parts {
				if _, err := strconv.ParseInt(part, 16, 32); err != nil {
					return fmt.Errorf("invalid hex value '%s': %w", part, err)
				}
			}
		}
		return nil
	}

	return validateTmuxKeyString(keys)
}

func validateTmuxKeyString(keys string) error {
	parts := strings.Fields(keys)

	for _, part := range parts {
		if err := validateSingleKey(part); err != nil {
			return fmt.Errorf("invalid key '%s': %w", part, err)
		}
	}

	return nil
}

func validateSingleKey(key string) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}

	if strings.Contains(key, "-") && len(key) > 2 {
		parts := strings.Split(key, "-")
		if len(parts) == 2 {
			modifier := parts[0]
			keyPart := parts[1]

			switch modifier {
			case "C", "^":
			case "M":
			case "S":
			default:
				return fmt.Errorf("invalid modifier '%s'", modifier)
			}

			if keyPart == "" {
				return fmt.Errorf("missing key after modifier")
			}
		}
	}

	if isSpecialKey(key) {
		return nil
	}

	return nil
}

func isSpecialKey(key string) bool {
	specialKeys := map[string]bool{
		"Up": true, "Down": true, "Left": true, "Right": true,
		"Home": true, "End": true, "PageUp": true, "PageDown": true,
		"PPage": true, "NPage": true,
		"F1": true, "F2": true, "F3": true, "F4": true, "F5": true, "F6": true,
		"F7": true, "F8": true, "F9": true, "F10": true, "F11": true, "F12": true,
		"F13": true, "F14": true, "F15": true, "F16": true, "F17": true, "F18": true,
		"F19": true, "F20": true,
		"Enter": true, "Return": true, "Tab": true, "BTab": true,
		"Escape": true, "Space": true, "Backspace": true, "Delete": true,
		"Insert": true, "DC": true, "IC": true,
		"MouseDown1": true, "MouseDown2": true, "MouseDown3": true,
		"MouseUp1": true, "MouseUp2": true, "MouseUp3": true,
		"MouseDrag1": true, "MouseDrag2": true, "MouseDrag3": true,
		"WheelUp": true, "WheelDown": true,
	}

	return specialKeys[key]
}

func parseKeyString(keys string, literal bool) []string {
	if literal {
		return []string{keys}
	}

	return strings.Fields(keys)
}
