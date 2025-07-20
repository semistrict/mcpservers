package tmuxmcp

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCaptureTool_Handle_BasicCapture(t *testing.T) {
	sessionName, err := createUniqueSession(t.Context(), "test-capture-basic", []string{"bash"})
	if !assert.NoError(t, err, "Failed to create unique session") {
		return
	}

	err = sendKeysToSession(t.Context(), SendKeysOptions{
		SessionName: sessionName,
		Keys:        "echo 'test output'",
		Enter:       true,
	})
	if !assert.NoError(t, err, "Failed to send keys to session") {
		return
	}

	tool := &CaptureTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
	}

	result, err := tool.Handle(t.Context())

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
	sessionName, err := createUniqueSession(t.Context(), "test-capture-change", []string{"bash"})
	if err != nil {
		t.Fatalf("Could not create tmux session: %v", err)
	}

	// Send initial content
	err = sendKeysToSession(t.Context(), SendKeysOptions{
		SessionName: sessionName,
		Keys:        "echo 'initial content'",
		Enter:       true,
	})

	assert.NoError(t, err)

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get initial hash
	captureResult, err := capture(t.Context(), captureOptions{Prefix: sessionName})
	if err != nil {
		t.Fatalf("Failed to get initial capture: %v", err)
	}
	initialHash := captureResult.Hash

	// Send new content to change the session
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = sendKeysToSession(t.Context(), SendKeysOptions{
			SessionName: sessionName,
			Keys:        "echo 'changed content'",
			Enter:       true,
		})
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
	sessionName, err := createUniqueSession(t.Context(), "test-capture-timeout", []string{"bash"})
	if err != nil {
		t.Fatalf("Could not create tmux session: %v", err)
	}

	// Get initial hash
	captureResult, err := capture(t.Context(), captureOptions{Prefix: sessionName})
	if err != nil {
		t.Fatalf("Failed to get initial capture: %v", err)
	}
	initialHash := captureResult.Hash

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
	sessionName, err := createUniqueSession(t.Context(), "test-capture-default-timeout", []string{"bash"})
	if err != nil {
		t.Fatalf("Could not create tmux session: %v", err)
	}

	// Get the actual current hash first
	captureResult, err := capture(t.Context(), captureOptions{Prefix: sessionName})
	if err != nil {
		t.Fatalf("Failed to get current capture: %v", err)
	}
	currentHash := captureResult.Hash

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
