package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"os/exec"
	"time"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*CaptureTool]())
}

type CaptureTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_capture" title:"Capture Tmux Session" description:"Capture output from tmux session with content hash" destructive:"false"`
	SessionTool
	WaitForChange string  `json:"wait_for_change" description:"Optional hash to wait for content to change from"`
	Timeout       float64 `json:"timeout" description:"Maximum seconds to wait for content change (default: 10)"`
}

func (t *CaptureTool) Handle(ctx context.Context) (interface{}, error) {
	sessionName, err := resolveSession(t.Prefix, t.Session)
	if err != nil {
		return nil, fmt.Errorf("error capturing session: %v", err)
	}

	// If WaitForChange is specified, wait for content to change from that hash
	if t.WaitForChange != "" {
		timeout := t.Timeout
		if timeout == 0 {
			timeout = 10 // default 10 seconds
		}

		result, err := t.waitForHashChange(sessionName, t.WaitForChange, timeout)
		if err != nil {
			return nil, fmt.Errorf("error waiting for content change: %v", err)
		}
		return result, nil
	}

	// Standard capture without waiting
	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error capturing session: failed to capture session %s: %v", sessionName, err)
	}

	formatted := formatOutput(string(output))
	hash := calculateHash(string(output))

	return fmt.Sprintf("Session: %s\nHash: %s\n\n%s", sessionName, hash, formatted), nil
}

func (t *CaptureTool) waitForHashChange(sessionName, expectedHash string, maxWait float64) (interface{}, error) {
	timeout := time.After(time.Duration(maxWait) * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			// Return current state even if it hasn't changed
			cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
			output, err := cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("failed to capture session after timeout: %v", err)
			}
			formatted := formatOutput(string(output))
			hash := calculateHash(string(output))
			return fmt.Sprintf("Session: %s\nHash: %s (unchanged after %.1f seconds)\n\n%s", sessionName, hash, maxWait, formatted), nil

		case <-ticker.C:
			cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
			output, err := cmd.Output()
			if err != nil {
				continue // Skip this iteration if capture fails
			}

			currentHash := calculateHash(string(output))
			if currentHash != expectedHash {
				// Content has changed!
				formatted := formatOutput(string(output))
				return fmt.Sprintf("Session: %s\nHash: %s (changed from %s)\n\n%s", sessionName, currentHash, expectedHash, formatted), nil
			}
		}
	}
}
