package tmuxmcp

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCaptureTool_Handle_BasicCapture(t *testing.T) {
	// Create a test session that stays alive
	sessionName := "test-capture-basic"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: could not create tmux session: %v", err)
	}
	defer killSession(sessionName)

	// Send some content to the session
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'test output'", "Enter").Run()

	// Wait for command to complete
	time.Sleep(200 * time.Millisecond)

	tool := &CaptureTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", result)
	}

	if !strings.Contains(resultStr, "Session: "+sessionName) {
		t.Errorf("Expected session name in output, got: %s", resultStr)
	}

	if !strings.Contains(resultStr, "Hash: ") {
		t.Errorf("Expected hash in output, got: %s", resultStr)
	}
}

func TestCaptureTool_Handle_WaitForChange_ContentChanges(t *testing.T) {
	// Create a test session that will change content
	sessionName := "test-capture-change"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: could not create tmux session: %v", err)
	}
	defer killSession(sessionName)

	// Send initial content
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'initial content'", "Enter").Run()

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get initial hash
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	initialOutput, err := captureCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get initial capture: %v", err)
	}
	initialHash := calculateHash(string(initialOutput))

	// Send new content to change the session
	go func() {
		time.Sleep(200 * time.Millisecond)
		exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'changed content'", "Enter").Run()
	}()

	tool := &CaptureTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
		WaitForChange: initialHash,
		Timeout:       2.0,
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", result)
	}

	if !strings.Contains(resultStr, "changed from") {
		t.Errorf("Expected 'changed from' in output indicating content changed, got: %s", resultStr)
	}

	if !strings.Contains(resultStr, initialHash) {
		t.Errorf("Expected initial hash %s in output, got: %s", initialHash, resultStr)
	}
}

func TestCaptureTool_Handle_WaitForChange_Timeout(t *testing.T) {
	// Create a test session that won't change
	sessionName := "test-capture-timeout"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: could not create tmux session: %v", err)
	}
	defer killSession(sessionName)

	// Send static content
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'static content'", "Enter").Run()

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get initial hash
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	initialOutput, err := captureCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get initial capture: %v", err)
	}
	initialHash := calculateHash(string(initialOutput))

	tool := &CaptureTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
		WaitForChange: initialHash,
		Timeout:       0.5, // Short timeout for test
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", result)
	}

	if !strings.Contains(resultStr, "unchanged after") {
		t.Errorf("Expected 'unchanged after' in output indicating timeout, got: %s", resultStr)
	}
}

func TestCaptureTool_Handle_WaitForChange_DefaultTimeout(t *testing.T) {
	// Create a test session
	sessionName := "test-capture-default-timeout"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: could not create tmux session: %v", err)
	}
	defer killSession(sessionName)

	// Send some content
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'test'", "Enter").Run()

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get the actual current hash first
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	currentOutput, err := captureCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get current capture: %v", err)
	}
	currentHash := calculateHash(string(currentOutput))

	tool := &CaptureTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
		WaitForChange: currentHash, // Use actual current hash so it won't change
		Timeout:       0.5,         // Short timeout for test
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	// Should get a result with timeout message
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", result)
	}

	if !strings.Contains(resultStr, "unchanged after") {
		t.Errorf("Expected timeout message, got: %s", resultStr)
	}
}
