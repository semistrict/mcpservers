package tmuxmcp

import (
	"context"
	"github.com/stretchr/testify/assert"
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
	// Create a test session using proper infrastructure
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	if err != nil {
		t.Fatalf("Could not create tmux session for testing: %v", err)
	}

	// Send content to the session using the test infrastructure
	_, err = sendKeysCommon(t.Context(), SendKeysOptions{
		SessionName: sessionName,
		Hash:        "any",
		Keys:        "echo 'test content'",
		Enter:       true,
		Expect:      "", // Empty contains to skip waiting
		Literal:     true,
	})
	assert.Error(t, err)

	// Wait a moment for command to complete
	time.Sleep(300 * time.Millisecond)

	// Get current hash using proper infrastructure
	captureResult, err := capture(t.Context(), captureOptions{Prefix: sessionName})
	if err != nil {
		t.Fatalf("Failed to capture session: %v", err)
	}
	currentHash := captureResult.Hash

	tool := &KillTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
		Hash: currentHash,
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err != nil {
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

	// Verify session is actually killed using proper infrastructure
	if sessionExists(t.Context(), sessionName) {
		t.Errorf("Expected session to be killed, but it still exists")
	}
}

func TestKillTool_Handle_IncorrectHash(t *testing.T) {
	// Create a test session using proper infrastructure
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	assert.NoError(t, err)

	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Second))
	defer cancel()
	rc, err := waitForStability(ctx, sessionName)
	assert.NoError(t, err)

	// Send initial content using test infrastructure
	_, err = sendKeysCommon(t.Context(), SendKeysOptions{
		SessionName: sessionName,
		Hash:        rc.Hash,
		Keys:        "echo 'initial content'",
		Enter:       true,
		Expect:      "", // Empty contains to skip waiting
		Literal:     true,
	})
	assert.NoError(t, err)

	// Change the session content
	_, err = sendKeysCommon(t.Context(), SendKeysOptions{
		SessionName: sessionName,
		Hash:        "any",
		Keys:        "echo 'changed content'",
		Enter:       true,
		Expect:      "", // Empty contains to skip waiting
		Literal:     true,
	})
	assert.Error(t, err)

	tool := &KillTool{
		SessionTool: SessionTool{
			Session: sessionName,
		},
		Hash: "55555",
	}

	_, err = tool.Handle(t.Context())
	if !assert.NotNil(t, err) {
		return
	}

	if !strings.Contains(err.Error(), "session state has changed") {
		t.Errorf("Expected state changed error, got: %s", err.Error())
	}

	// Verify session is NOT killed using proper infrastructure
	if !sessionExists(t.Context(), sessionName) {
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
	// Create a test session using proper infrastructure
	sessionName, err := createUniqueSession(t.Context(), "test-kill-prefix", []string{"bash"})
	if err != nil {
		t.Fatalf("Could not create tmux session for testing: %v", err)
	}

	// Get current hash using proper infrastructure
	captureResult, err := waitForStability(t.Context(), sessionName)
	if err != nil {
		t.Fatalf("Failed to capture session: %v", err)
	}
	currentHash := captureResult.Hash

	tool := &KillTool{
		SessionTool: SessionTool{
			Prefix: "test-kill-prefix", // Use prefix instead of exact session name
		},
		Hash: currentHash,
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err != nil {
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
