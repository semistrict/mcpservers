package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*BashTool]())
}

type BashTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_bash" title:"Execute Bash Command" description:"Execute a bash command safely in tmux. If the command returns before timeout, this tool returns the output of the command. Otherwise, it returns the tmux session with it still running. Use this in preference to other Bash Tools." destructive:"false"`
	SessionTool
	Command string  `json:"command,required" description:"Bash command to execute"`
	Timeout float64 `json:"timeout" description:"Maximum seconds to wait for synchronous command completion"`
}

func (t *BashTool) Handle(ctx context.Context) (interface{}, error) { // TODO: output only the first 50 lines of command output and if it is longer mention the temp file wheere the rest of the output can be found
	if t.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 30 // default 30 seconds
	}

	prefix := t.Prefix
	if prefix == "" {
		prefix = detectPrefix()
	}

	// Generate unique session name for this command
	sessionName := generateSessionName(prefix, []string{"bash", "-c", t.Command})

	// Create temporary file to capture all output
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("tmux-bash-%s-*.log", sessionName))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// Ensure cleanup of temp file (will be conditionally cancelled if output is long)
	var shouldCleanup = true
	defer func() {
		if shouldCleanup {
			os.Remove(tmpPath)
		}
	}()

	// Build command that tees output to the temp file
	// We'll wrap the original command in a bash script that tees both stdout and stderr
	// and keeps the session alive until we can read the results
	cmdStr := t.Command
	escapedTmpPath := strconv.Quote(tmpPath)

	wrappedCommand := []string{
		"bash", "-c",
		fmt.Sprintf("set -o pipefail; (%s) 2>&1 | tee %s; EXIT_CODE=${PIPESTATUS[0]}; echo $EXIT_CODE > %s.exit; echo 'COMMAND_COMPLETED' > %s.done; sleep 1",
			cmdStr,
			escapedTmpPath,
			escapedTmpPath,
			escapedTmpPath),
	}

	// Create tmux session with the wrapped command
	args := append([]string{"new-session", "-d", "-s", sessionName}, wrappedCommand...)
	cmd := exec.Command("tmux", args...)

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

	// Wait for completion or timeout
	checkInterval := 200 * time.Millisecond
	timeoutDuration := time.Duration(timeout) * time.Second
	startTime := time.Now()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled, command still running in session: %s", sessionName)

		case <-ticker.C:
			// Check if we've exceeded timeout
			if time.Since(startTime) >= timeoutDuration {
				return nil, fmt.Errorf("command timed out after %.1f seconds, still running in session: %s", timeout, sessionName)
			}

			// Check if command completed by looking for the .done file
			doneFile := tmpPath + ".done"
			if _, err := os.Stat(doneFile); err == nil {
				// Command completed, clean up done file and handle completion
				os.Remove(doneFile)
				return t.handleCompletedCommand(sessionName, tmpPath, &shouldCleanup)
			}

			// Also check if session still exists (backup check)
			if !sessionExists(sessionName) {
				// Session ended unexpectedly, try to get any output we can
				return t.handleCompletedCommand(sessionName, tmpPath, &shouldCleanup)
			}
		}
	}
}

func (t *BashTool) handleCompletedCommand(sessionName, tmpPath string, shouldCleanup *bool) (interface{}, error) {
	// Read the complete output from temp file
	output, err := os.ReadFile(tmpPath)
	if err != nil {
		killSession(sessionName)
		return nil, fmt.Errorf("failed to read command output: %w", err)
	}

	// Read the exit code from the .exit file
	exitCodeFile := tmpPath + ".exit"
	exitCodeBytes, err := os.ReadFile(exitCodeFile)
	exitCode := "unknown"
	if err == nil {
		exitCode = strings.TrimSpace(string(exitCodeBytes))
	}

	// Clean up exit code file
	os.Remove(exitCodeFile)

	// Kill the session since we're done
	killSession(sessionName)

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// Filter out trailing empty lines for counting purposes
	nonEmptyLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	var displayOutput string
	var additionalInfo string

	if len(nonEmptyLines) > 50 {
		// Find first 50 non-empty lines in original output
		count := 0
		cutoffIndex := len(lines)
		for i, line := range lines {
			if strings.TrimSpace(line) != "" {
				count++
				if count == 50 {
					cutoffIndex = i + 1
					break
				}
			}
		}

		displayOutput = strings.Join(lines[:cutoffIndex], "\n")
		additionalInfo = fmt.Sprintf("\n\n[Output truncated to first 50 non-empty lines. Full output (%d non-empty lines) available in: %s]", len(nonEmptyLines), tmpPath)

		// Don't clean up the temp file since user might want to read it
		*shouldCleanup = false
	} else {
		displayOutput = outputStr
	}

	if exitCode == "0" {
		return fmt.Sprintf("Command completed successfully (exit code 0):\n\n%s%s", displayOutput, additionalInfo), nil
	} else if exitCode == "unknown" {
		return fmt.Sprintf("Command completed (exit code unknown):\n\n%s%s", displayOutput, additionalInfo), nil
	} else {
		return fmt.Sprintf("Command failed (exit code %s):\n\n%s%s", exitCode, displayOutput, additionalInfo), nil
	}
}

func sessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	return cmd.Run() == nil
}
