package tmuxmcp

import (
	"context"
	"fmt"
	"os/exec"
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
		return string(output), fmt.Errorf("tmux command failed: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}