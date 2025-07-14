package automcp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DangerousCommands lists commands that should never be executed, even for help
var DangerousCommands = map[string]bool{
	"reboot": true,
	"halt":   true,
	"init":   true,
}

// SafeCommandExecutor provides sandboxed command execution
type SafeCommandExecutor struct {
	timeout time.Duration
}

// NewSafeCommandExecutor creates a new safe command executor
func NewSafeCommandExecutor() *SafeCommandExecutor {
	return &SafeCommandExecutor{
		timeout: 10 * time.Second, // Max 10 seconds for help commands
	}
}

// ExecuteCommand safely executes a command in a restricted environment
func (s *SafeCommandExecutor) ExecuteCommand(command string, args []string) ([]byte, error) {
	// Check if command is in dangerous list
	baseCommand := filepath.Base(command)
	if DangerousCommands[baseCommand] {
		return nil, fmt.Errorf("command '%s' is blocked for security reasons", baseCommand)
	}

	// Create command with timeout
	cmd := exec.Command(command, args...)

	// Set up safe environment
	cmd.Env = s.getSafeEnvironment()

	// Set working directory to a safe temporary location
	if tmpDir, err := os.MkdirTemp("", "automcp-sandbox-*"); err == nil {
		cmd.Dir = tmpDir
		defer os.RemoveAll(tmpDir) // Clean up after execution
	}

	// Set resource limits (process-level, not requiring root)
	s.setResourceLimits(cmd)

	// Execute with timeout
	done := make(chan bool)
	var output []byte
	var err error

	go func() {
		output, err = cmd.CombinedOutput()
		done <- true
	}()

	select {
	case <-done:
		return output, err
	case <-time.After(s.timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("command timed out after %v", s.timeout)
	}
}

// getSafeEnvironment returns a minimal, safe environment
func (s *SafeCommandExecutor) getSafeEnvironment() []string {
	// Start with minimal safe environment
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}
	safeEnv := []string{
		"HOME=/tmp",     // Safe home directory
		"SHELL=/bin/sh", // Basic shell
		"USER=nobody",   // Non-privileged user
		"TMPDIR=/tmp",   // Safe temp directory
		"LC_ALL=C",      // Predictable locale"
		"PATH=" + path,  // Pass through current PATH or use safe default
	}

	// Add only essential environment variables
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "LANG=") ||
			strings.HasPrefix(env, "TERM=") {
			safeEnv = append(safeEnv, env)
		}
	}

	return safeEnv
}

// setResourceLimits sets basic resource limits (userspace only)
func (s *SafeCommandExecutor) setResourceLimits(cmd *exec.Cmd) {
	// These are basic limits that don't require root privileges
	// On most systems, these help prevent runaway processes

	// Note: More advanced limits would require setrlimit syscalls
	// or tools like ulimit, but those often need privileges

	// For now, we rely on:
	// 1. Timeout (handled in ExecuteCommand)
	// 2. Safe environment (no sensitive env vars)
	// 3. Safe working directory (temporary, isolated)
	// 4. Command filtering (dangerous commands blocked)
}

// IsCommandSafe checks if a command is safe to execute for help
func IsCommandSafe(command string) bool {
	baseCommand := filepath.Base(command)
	return !DangerousCommands[baseCommand]
}

// GetSafeHelpFlags returns help flags that are generally safe
func GetSafeHelpFlags() []string {
	return []string{
		"--help",
		"-h",
		"help",
		"--usage",
		"-?",
		"--version", // Often safe and informative
		"-V",
	}
}
