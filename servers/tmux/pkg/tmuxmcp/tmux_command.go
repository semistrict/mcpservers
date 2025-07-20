package tmuxmcp

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
)

// runTmuxCommand creates and executes a tmux command with the given context
// Returns the combined stderr and stdout as a string
// If command exits non-zero, error includes the output
func runTmuxCommand(ctx context.Context, args ...string) (string, error) {
	var cmd *exec.Cmd
	if testSocketPath != "" {
		// Prepend socket args
		allArgs := append([]string{"-S", testSocketPath}, args...)
		cmd = exec.CommandContext(ctx, "tmux", allArgs...)
	} else {
		cmd = exec.CommandContext(ctx, "tmux", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("tmux command not found, please ensure tmux is installed: %w", err)
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return "", fmt.Errorf("(%d) %s", exitError.ExitCode(), output)
		}
		return "", err
	}
	return string(output), nil
}

var createdSessions = make(map[string]struct{})
var createdSessionsMu sync.Mutex

func newSession(ctx context.Context, sessionName string, command []string, environment map[string]string) error {
	createdSessionsMu.Lock()
	defer createdSessionsMu.Unlock()
	if _, exists := createdSessions[sessionName]; exists {
		return fmt.Errorf("session %s already exists", sessionName)
	}
	// Create a new tmux session with the given name and command
	args := []string{"new-session", "-d", "-s", sessionName}

	// Add environment variables using -e flag
	for k, v := range environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	if len(command) > 0 {
		args = append(args, command...)
	}

	output, err := runTmuxCommand(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to create tmux session: %w\nOutput: %s", err, output)
	}

	createdSessions[sessionName] = struct{}{}
	return nil
}
