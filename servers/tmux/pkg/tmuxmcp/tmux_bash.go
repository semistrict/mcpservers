package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*BashTool]())
}

type BashTool struct {
	_                mcpcommon.ToolInfo `name:"tmux_bash" title:"Execute Bash Command" description:"Execute a bash command safely in a new tmux and return its output (usually not necessary to capture output again). If the command completes within timeout, returns the full output. If it times out, returns the session name where it's still running. Use this in preference to other Bash Tools." destructive:"true"`
	Prefix           string             `json:"prefix" description:"Session name prefix (auto-detected from git repo if not provided)"`
	Command          string             `json:"command,required" description:"Bash command to execute"`
	WorkingDirectory string             `json:"working_directory,required" description:"Directory to execute the command in"`
	Timeout          float64            `json:"timeout" description:"Maximum seconds to wait for synchronous command completion"`
}

func (t *BashTool) Handle(ctx context.Context) (interface{}, error) { // TODO: output only the first 50 lines of command output and if it is longer mention the temp file wheere the rest of the output can be found
	if t.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	if t.WorkingDirectory == "" {
		return nil, fmt.Errorf("working_directory is required")
	}

	timeout := t.Timeout
	if timeout == 0 {
		timeout = 30 // default 30 seconds
	}

	prefix := t.Prefix
	if prefix == "" {
		prefix = detectPrefix()
	}

	// Create temporary file to capture all output
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("tmux-bash-%s-*.log", prefix))
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
	workingDir := strconv.Quote(t.WorkingDirectory)
	escapedTmpPath := strconv.Quote(tmpPath)

	wrappedCommand := []string{
		"bash", "-c",
		fmt.Sprintf("set -o pipefail; cd %s && (%s) 2>&1 | tee %s; EXIT_CODE=${PIPESTATUS[0]}; echo $EXIT_CODE > %s.exit; echo 'COMMAND_COMPLETED' > %s.done; sleep 1",
			workingDir,
			cmdStr,
			escapedTmpPath,
			escapedTmpPath,
			escapedTmpPath),
	}

	// Create tmux session with the wrapped command
	sessionName, err := createUniqueSession(ctx, prefix, wrappedCommand)
	if err != nil {
		return nil, err
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
				return t.handleCompletedCommand(ctx, sessionName, tmpPath, &shouldCleanup)
			}

			// Also check if session still exists (backup check)
			if !sessionExists(ctx, sessionName) {
				// Session ended unexpectedly, try to get any output we can
				return t.handleCompletedCommand(ctx, sessionName, tmpPath, &shouldCleanup)
			}
		}
	}
}

func (t *BashTool) handleCompletedCommand(ctx context.Context, sessionName, tmpPath string, shouldCleanup *bool) (interface{}, error) {
	// Read the complete output from temp file
	output, err := os.ReadFile(tmpPath)
	if err != nil {
		killSession(ctx, sessionName)
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
	killSession(ctx, sessionName)

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

func sessionExists(ctx context.Context, sessionName string) bool {
	_, err := runTmuxCommand(ctx, "has-session", "-t", sessionName)
	return err == nil
}
