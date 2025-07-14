package tmuxmcp

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestKillTool_Handle_RequiresHash(t *testing.T) {
	tool := &KillTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		// No hash provided
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err == nil {
		t.Fatalf("Expected error when no hash provided, got result: %v", result)
	}

	if !strings.Contains(err.Error(), "hash is required for safety") {
		t.Errorf("Expected hash required error, got: %s", err.Error())
	}
}

func TestKillTool_Handle_CorrectHash(t *testing.T) {
	// Create a test session
	sessionName := "test-kill-correct-hash"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: could not create tmux session: %v", err)
	}

	// Send content to the session
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'test content'", "Enter").Run()

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get current hash
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := captureCmd.Output()
	if err != nil {
		killSession(sessionName) // Cleanup
		t.Fatalf("Failed to capture session: %v", err)
	}
	currentHash := calculateHash(string(output))

	tool := &KillTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
		Hash: currentHash,
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err != nil {
		killSession(sessionName) // Cleanup on error
		t.Fatalf("Expected no error with correct hash, got: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", result)
	}

	if !strings.Contains(resultStr, "killed successfully") {
		t.Errorf("Expected success message, got: %s", resultStr)
	}

	if !strings.Contains(resultStr, sessionName) {
		t.Errorf("Expected session name in result, got: %s", resultStr)
	}

	// Verify session is actually killed
	checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if checkCmd.Run() == nil {
		t.Errorf("Expected session to be killed, but it still exists")
	}
}

func TestKillTool_Handle_IncorrectHash(t *testing.T) {
	// Create a test session
	sessionName := "test-kill-incorrect-hash"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: could not create tmux session: %v", err)
	}
	defer killSession(sessionName) // Ensure cleanup

	// Send initial content
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'initial content'", "Enter").Run()

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get initial hash
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := captureCmd.Output()
	if err != nil {
		t.Fatalf("Failed to capture session: %v", err)
	}
	initialHash := calculateHash(string(output))

	// Change the session content
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'changed content'", "Enter").Run()
	time.Sleep(100 * time.Millisecond) // Wait for change to take effect

	tool := &KillTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
		Hash: initialHash, // Use old hash - should fail
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err == nil {
		t.Fatalf("Expected error with incorrect hash, got result: %v", result)
	}

	if !strings.Contains(err.Error(), "session state has changed") {
		t.Errorf("Expected state changed error, got: %s", err.Error())
	}

	if !strings.Contains(err.Error(), initialHash) {
		t.Errorf("Expected initial hash in error message, got: %s", err.Error())
	}

	// Verify session is NOT killed
	checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if checkCmd.Run() != nil {
		t.Errorf("Expected session to still exist after failed kill, but it was killed")
	}
}

func TestKillTool_Handle_SessionNotFound(t *testing.T) {
	tool := &KillTool{
		SessionTool: SessionTool{
			Session: "nonexistent-session",
		},
		Hash: "somehash",
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err == nil {
		t.Fatalf("Expected error for nonexistent session, got result: %v", result)
	}

	// Should get an error about session not found or capture failure
	if !strings.Contains(err.Error(), "failed to verify session state") &&
		!strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected session not found error, got: %s", err.Error())
	}
}

func TestKillTool_Handle_PrefixResolution(t *testing.T) {
	// Create a test session with prefix
	prefix := "test-kill-prefix"
	sessionName := prefix + "-session-12345"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: could not create tmux session: %v", err)
	}

	// Send content to the session
	exec.Command("tmux", "send-keys", "-t", sessionName, "echo 'test'", "Enter").Run()

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get current hash
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := captureCmd.Output()
	if err != nil {
		killSession(sessionName)
		t.Fatalf("Failed to capture session: %v", err)
	}
	currentHash := calculateHash(string(output))

	tool := &KillTool{
		SessionTool: SessionTool{
			Prefix: prefix, // Use prefix instead of exact session name
		},
		Hash: currentHash,
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err != nil {
		killSession(sessionName) // Cleanup on error
		t.Fatalf("Expected no error with prefix resolution, got: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", result)
	}

	if !strings.Contains(resultStr, "killed successfully") {
		t.Errorf("Expected success message, got: %s", resultStr)
	}
}
