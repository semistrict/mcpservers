package tmux

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

type Client struct{}

// NewSessionOptions contains options for creating a new tmux session
type NewSessionOptions struct {
	Command       []string
	Prefix        string
	Expect        string
	KillOthers    bool
	AllowMultiple bool
	MaxWait       float64
}

// NewSessionResult contains the result of creating a new session
type NewSessionResult struct {
	SessionName string
	Output      string
	Hash        string
}

// CaptureOptions contains options for capturing session output
type CaptureOptions struct {
	Prefix  string
	Session string
}

// CaptureResult contains the result of capturing session output
type CaptureResult struct {
	SessionName string
	Output      string
	Hash        string
}

// SendKeysOptions contains options for sending keys to a session
type SendKeysOptions struct {
	Hash    string  // Required hash from session capture for safety verification
	Keys    string  // Keys to send - supports tmux syntax: literals, C- (Ctrl), M- (Alt), S- (Shift), special keys (Enter, F1-F12, Up, Down, etc.)
	Prefix  string  // Session prefix for auto-detection (optional if Session specified)
	Session string  // Specific session name (optional if using prefix auto-detection)
	Enter   bool    // Whether to send Enter key after the keys
	Expect  string  // Expected text to appear in output after sending keys (optional)
	MaxWait float64 // Maximum seconds to wait for expected text or stability (default: 10s for stability, 60s for expect)
	Literal bool    // Use -l flag: treat keys as literal UTF-8 characters (no special key interpretation)
	Hex     bool    // Use -H flag: treat keys as hexadecimal ASCII character codes
}

// SendKeysResult contains the result of sending keys
type SendKeysResult struct {
	SessionName string
	Output      string
	Hash        string
}

// KillOptions contains options for killing a session
type KillOptions struct {
	Prefix  string
	Session string
}

// AttachOptions contains options for attaching to a session
type AttachOptions struct {
	Prefix    string
	Session   string
	ReadWrite bool
	NewWindow bool
}

// Constants for timeouts
const (
	DefaultWaitTimeout = 10
	ExpectWaitTimeout  = 60
	NoOutputTimeout    = 20
	CheckInterval      = 200 * time.Millisecond
	StabilityThreshold = 500 * time.Millisecond
)

// NewSession creates a new tmux session
func (c *Client) NewSession(opts NewSessionOptions) (*NewSessionResult, error) {
	// Auto-detect prefix if not provided
	if opts.Prefix == "" {
		opts.Prefix = c.detectPrefix()
	}

	// Handle kill-others flag
	if opts.KillOthers {
		sessions, err := c.findSessionsByPrefix(opts.Prefix)
		if err == nil {
			for _, session := range sessions {
				c.killSession(session)
			}
		}
	}

	// Check for existing sessions if not allowing multiple
	if !opts.AllowMultiple {
		existing, err := c.findSessionsByPrefix(opts.Prefix)
		if err == nil && len(existing) > 0 {
			return nil, fmt.Errorf("session with prefix '%s' already exists: %s. Use --allow-multiple or --kill-others", opts.Prefix, existing[0])
		}
	}

	// Generate session name
	sessionName := c.generateSessionName(opts.Prefix, opts.Command)

	// Create the session
	var cmd *exec.Cmd
	if len(opts.Command) > 0 {
		args := append([]string{"new-session", "-d", "-s", sessionName}, opts.Command...)
		cmd = exec.Command("tmux", args...)
	} else {
		cmd = exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	}

	if err := cmd.Run(); err != nil {
		// If tmux fails, try cleaning up tmp files and retry once
		// This addresses the issue described in https://github.com/tmux/tmux/issues/2376
		if err := c.cleanupTmuxTempFiles(); err == nil {
			// Retry after cleanup
			if retryErr := cmd.Run(); retryErr == nil {
				// Success on retry
			} else {
				return nil, fmt.Errorf("failed to create session even after cleanup: %w", retryErr)
			}
		} else {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Wait for output if expect string provided or default stability
	var output string
	var hash string
	if opts.Expect != "" {
		result, err := c.waitForExpected(sessionName, opts.Expect, opts.MaxWait)
		if err != nil {
			return nil, err
		}
		output = result.Output
		hash = result.Hash
	} else {
		result, err := c.waitForStability(sessionName, opts.MaxWait)
		if err != nil {
			return nil, err
		}
		output = result.Output
		hash = result.Hash
	}

	return &NewSessionResult{
		SessionName: sessionName,
		Output:      output,
		Hash:        hash,
	}, nil
}

// Capture captures the current output of a tmux session
func (c *Client) Capture(opts CaptureOptions) (*CaptureResult, error) {
	sessionName, err := c.resolveSession(opts.Prefix, opts.Session)
	if err != nil {
		return nil, err
	}

	// Capture the pane content
	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to capture session %s: %w", sessionName, err)
	}

	formatted := c.formatOutput(string(output))
	hash := c.calculateHash(string(output))

	return &CaptureResult{
		SessionName: sessionName,
		Output:      formatted,
		Hash:        hash,
	}, nil
}

// SendKeys sends keys to a tmux session with hash verification
func (c *Client) SendKeys(opts SendKeysOptions) (*SendKeysResult, error) {
	// Validate required parameters
	if opts.Hash == "" {
		return nil, fmt.Errorf("hash is required for safety. Please capture the session first with tmux_capture to get the current hash, then use that hash in tmux_send_keys")
	}

	if opts.Keys == "" {
		return nil, fmt.Errorf("keys parameter is required. Specify the keys to send to the session")
	}

	sessionName, err := c.resolveSession(opts.Prefix, opts.Session)
	if err != nil {
		return nil, err
	}

	// Verify current hash
	current, err := c.Capture(CaptureOptions{Session: sessionName})
	if err != nil {
		return nil, fmt.Errorf("failed to verify session state: %w", err)
	}

	if current.Hash != opts.Hash {
		return nil, fmt.Errorf("session state has changed. Expected hash %s, got %s. Please capture current output first and carefully consider whether the sent keys still make sense.", opts.Hash, current.Hash)
	}

	// Send the keys
	args := []string{"send-keys", "-t", sessionName}

	// Add flags based on options
	if opts.Literal {
		args = append(args, "-l")
	}
	if opts.Hex {
		args = append(args, "-H")
	}

	// Add the keys - support for tmux's full key syntax
	if opts.Keys != "" {
		// Parse and validate keys before sending
		if err := c.validateKeyString(opts.Keys, opts.Literal, opts.Hex); err != nil {
			return nil, fmt.Errorf("invalid key string: %w", err)
		}

		// Split keys by spaces to handle multiple key arguments
		keyParts := c.parseKeyString(opts.Keys, opts.Literal)
		args = append(args, keyParts...)
	}

	if opts.Enter {
		args = append(args, "Enter")
	}

	cmd := exec.Command("tmux", args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to send keys to session %s: %w", sessionName, err)
	}

	// Wait for output
	var result *CaptureResult
	if opts.Expect != "" {
		result, err = c.waitForExpected(sessionName, opts.Expect, opts.MaxWait)
	} else {
		result, err = c.waitForStability(sessionName, opts.MaxWait)
	}
	if err != nil {
		return nil, err
	}

	return &SendKeysResult{
		SessionName: sessionName,
		Output:      result.Output,
		Hash:        result.Hash,
	}, nil
}

// List returns all tmux sessions, optionally filtered by prefix
func (c *Client) List(prefix string) ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// No sessions exist
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

// Kill kills a tmux session
func (c *Client) Kill(opts KillOptions) (string, error) {
	sessionName, err := c.resolveSession(opts.Prefix, opts.Session)
	if err != nil {
		return "", err
	}

	if err := c.killSession(sessionName); err != nil {
		return "", err
	}

	return sessionName, nil
}

// Attach attaches to a tmux session
func (c *Client) Attach(opts AttachOptions) (string, error) {
	sessionName, err := c.resolveSession(opts.Prefix, opts.Session)
	if err != nil {
		return "", err
	}

	if opts.NewWindow {
		// macOS iTerm integration
		return c.attachNewWindow(sessionName, opts.ReadWrite)
	}

	// Standard attach
	var cmd *exec.Cmd
	if opts.ReadWrite {
		cmd = exec.Command("tmux", "attach-session", "-t", sessionName)
	} else {
		cmd = exec.Command("tmux", "attach-session", "-r", "-t", sessionName)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to attach to session %s: %w", sessionName, err)
	}

	return sessionName, nil
}

// Helper functions

func (c *Client) detectPrefix() string {
	// Try to get git repository name
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "tmux"
	}

	repoPath := strings.TrimSpace(string(output))
	repoName := filepath.Base(repoPath)

	// Sanitize for tmux session name
	reg := regexp.MustCompile(`[^a-zA-Z0-9-_]`)
	sanitized := reg.ReplaceAllString(repoName, "-")

	return sanitized
}

func (c *Client) generateSessionName(prefix string, command []string) string {
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

	// Add random suffix
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%s-%d", prefix, cmdPart, timestamp%10000)
}

func (c *Client) resolveSession(prefix, session string) (string, error) {
	if session != "" {
		// Check if specific session exists
		sessions, err := c.List("")
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

	// Auto-detect prefix if not provided
	if prefix == "" {
		prefix = c.detectPrefix()
	}

	// Find sessions by prefix
	sessions, err := c.findSessionsByPrefix(prefix)
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

func (c *Client) findSessionsByPrefix(prefix string) ([]string, error) {
	sessions, err := c.List("")
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

func (c *Client) killSession(sessionName string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	return cmd.Run()
}

func (c *Client) formatOutput(output string) string {
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

func (c *Client) calculateHash(content string) string {
	if strings.TrimSpace(content) == "" {
		return "empty"
	}

	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)[:8]
}

func (c *Client) waitForStability(sessionName string, maxWait float64) (*CaptureResult, error) {
	if maxWait == 0 {
		maxWait = DefaultWaitTimeout
	}

	timeout := time.After(time.Duration(maxWait) * time.Second)
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	var lastOutput string
	var lastChange time.Time = time.Now()

	for {
		select {
		case <-timeout:
			result, _ := c.Capture(CaptureOptions{Session: sessionName})
			if result != nil {
				return result, nil
			}
			return nil, fmt.Errorf("timeout waiting for stability after %.1f seconds", maxWait)

		case <-ticker.C:
			result, err := c.Capture(CaptureOptions{Session: sessionName})
			if err != nil {
				continue
			}

			if result.Output != lastOutput {
				lastOutput = result.Output
				lastChange = time.Now()
			} else if time.Since(lastChange) >= StabilityThreshold {
				return result, nil
			}
		}
	}
}

func (c *Client) waitForExpected(sessionName, expected string, maxWait float64) (*CaptureResult, error) {
	if maxWait == 0 {
		maxWait = ExpectWaitTimeout
	}

	timeout := time.After(time.Duration(maxWait) * time.Second)
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	var lastOutput string
	var lastChange time.Time = time.Now()

	for {
		select {
		case <-timeout:
			result, _ := c.Capture(CaptureOptions{Session: sessionName})
			if result != nil {
				return result, fmt.Errorf("timeout waiting for '%s' after %.1f seconds", expected, maxWait)
			}
			return nil, fmt.Errorf("timeout waiting for '%s' after %.1f seconds", expected, maxWait)

		case <-ticker.C:
			result, err := c.Capture(CaptureOptions{Session: sessionName})
			if err != nil {
				continue
			}

			if strings.Contains(result.Output, expected) {
				return result, nil
			}

			if result.Output != lastOutput {
				lastOutput = result.Output
				lastChange = time.Now()
			} else if time.Since(lastChange) >= time.Duration(NoOutputTimeout)*time.Second {
				return result, fmt.Errorf("no new output for %d seconds while waiting for '%s'", NoOutputTimeout, expected)
			}
		}
	}
}

func (c *Client) attachNewWindow(sessionName string, readWrite bool) (string, error) {
	// macOS iTerm AppleScript integration
	mode := "-r"
	if readWrite {
		mode = ""
	}

	script := fmt.Sprintf(`
		tell application "iTerm"
			create window with default profile
			tell current session of current window
				write text "tmux attach-session %s -t %s"
			end tell
			activate
		end tell
	`, mode, sessionName)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to open new iTerm window: %w", err)
	}

	return sessionName, nil
}

// cleanupTmuxTempFiles removes tmux-related files from /tmp to address
// the issue described in https://github.com/tmux/tmux/issues/2376
func (c *Client) cleanupTmuxTempFiles() error {
	tmpDir := "/tmp"

	// Find all tmux-related files in /tmp
	pattern := filepath.Join(tmpDir, "tmux-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find tmux temp files: %w", err)
	}

	// Remove each tmux temp file/directory
	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil {
			// Log the error but continue with other files
			continue
		}
	}

	return nil
}

// validateKeyString validates the key string according to tmux syntax
func (c *Client) validateKeyString(keys string, literal, hex bool) error {
	if keys == "" {
		return fmt.Errorf("keys cannot be empty")
	}

	// If literal or hex mode, no special validation needed
	if literal || hex {
		if hex {
			// For hex mode, validate that it contains valid hex characters
			parts := strings.Fields(keys)
			for _, part := range parts {
				if _, err := strconv.ParseInt(part, 16, 32); err != nil {
					return fmt.Errorf("invalid hex value '%s': %w", part, err)
				}
			}
		}
		return nil
	}

	// For non-literal mode, validate tmux key syntax
	return c.validateTmuxKeyString(keys)
}

// validateTmuxKeyString validates tmux special key syntax
func (c *Client) validateTmuxKeyString(keys string) error {
	// Split by spaces to handle multiple keys
	parts := strings.Fields(keys)

	for _, part := range parts {
		if err := c.validateSingleKey(part); err != nil {
			return fmt.Errorf("invalid key '%s': %w", part, err)
		}
	}

	return nil
}

// validateSingleKey validates a single key according to tmux syntax
func (c *Client) validateSingleKey(key string) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}

	// Check for modifier prefixes
	if strings.Contains(key, "-") && len(key) > 2 {
		parts := strings.Split(key, "-")
		if len(parts) == 2 {
			modifier := parts[0]
			keyPart := parts[1]

			// Validate modifier
			switch modifier {
			case "C", "^": // Ctrl
			case "M": // Alt/Meta
			case "S": // Shift
			default:
				return fmt.Errorf("invalid modifier '%s'", modifier)
			}

			// Validate the key part
			if keyPart == "" {
				return fmt.Errorf("missing key after modifier")
			}
		}
	}

	// Check if it's a known special key
	if c.isSpecialKey(key) {
		return nil
	}

	// For regular characters, allow anything
	return nil
}

// isSpecialKey checks if the key is a known tmux special key
func (c *Client) isSpecialKey(key string) bool {
	specialKeys := map[string]bool{
		// Navigation keys
		"Up": true, "Down": true, "Left": true, "Right": true,
		"Home": true, "End": true, "PageUp": true, "PageDown": true,
		"PPage": true, "NPage": true,

		// Function keys
		"F1": true, "F2": true, "F3": true, "F4": true, "F5": true, "F6": true,
		"F7": true, "F8": true, "F9": true, "F10": true, "F11": true, "F12": true,
		"F13": true, "F14": true, "F15": true, "F16": true, "F17": true, "F18": true,
		"F19": true, "F20": true,

		// Other special keys
		"Enter": true, "Return": true, "Tab": true, "BTab": true,
		"Escape": true, "Space": true, "Backspace": true, "Delete": true,
		"Insert": true, "DC": true, "IC": true,

		// Mouse events
		"MouseDown1": true, "MouseDown2": true, "MouseDown3": true,
		"MouseUp1": true, "MouseUp2": true, "MouseUp3": true,
		"MouseDrag1": true, "MouseDrag2": true, "MouseDrag3": true,
		"WheelUp": true, "WheelDown": true,
	}

	return specialKeys[key]
}

// parseKeyString parses the key string into separate arguments for tmux
func (c *Client) parseKeyString(keys string, literal bool) []string {
	if literal {
		// In literal mode, send as single argument
		return []string{keys}
	}

	// In normal mode, split by spaces to handle multiple keys
	return strings.Fields(keys)
}
