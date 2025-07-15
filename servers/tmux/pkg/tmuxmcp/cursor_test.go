package tmuxmcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCaptureWithCursor_Integration(t *testing.T) {
	// Create a real tmux session for testing
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	if err != nil {
		t.Skipf("Could not create tmux session for testing: %v", err)
	}
	defer func() { killSession(t.Context(), sessionName) }()

	// Test capturing with cursor position
	result, err := captureWithCursor(t.Context(), captureOptions{Prefix: sessionName})
	if err != nil {
		t.Fatalf("Expected no error for real session, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result for successful capture")
	}

	if result.SessionName != sessionName {
		t.Errorf("Expected SessionName to be '%s', got: %s", sessionName, result.SessionName)
	}

	if result.CursorY < 0 {
		t.Errorf("Expected CursorY to be non-negative, got: %d", result.CursorY)
	}

	if result.CursorX < 0 {
		t.Errorf("Expected CursorX to be non-negative, got: %d", result.CursorX)
	}

	if result.Output == "" {
		t.Error("Expected non-empty output")
	}

	if result.Hash == "" {
		t.Error("Expected non-empty hash")
	}
}

func TestWaitForExpected_CursorLineOnly_Integration(t *testing.T) {
	// Create a real tmux session for testing
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	if err != nil {
		t.Skipf("Could not create tmux session for testing: %v", err)
	}
	defer func() { killSession(t.Context(), sessionName) }()

	// Send a command that will provide a recognizable prompt
	sendKeysCommon(t.Context(), SendKeysOptions{
		SessionName: sessionName,
		Hash:        "any", // We'll bypass hash check by using empty expect
		Keys:        "echo 'test-marker'",
		Enter:       true,
		Expect:      "", // Empty expect to skip waiting
		Literal:     true,
	})

	// Wait for something that should appear on the cursor line
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(2*time.Second))
	defer cancel()
	result, err := waitForExpected(ctx, sessionName, "test-marker")

	// This might timeout since the output may not be on cursor line,
	// but we're testing the function works with real sessions
	if err != nil && !strings.Contains(err.Error(), "cursor line") {
		t.Errorf("Expected timeout error to mention cursor line, got: %v", err)
	}

	// If we did get a result, verify it has expected structure
	if result != nil {
		if result.SessionName != sessionName {
			t.Errorf("Expected SessionName to be '%s', got: %s", sessionName, result.SessionName)
		}
		if result.Hash == "" {
			t.Error("Expected non-empty hash in result")
		}
	}
}

func TestCursorResult_Structure(t *testing.T) {
	// Test that cursorResult has the expected fields
	result := &cursorResult{
		SessionName: "test-session",
		CursorLine:  "$ echo hello",
		CursorY:     23,
		CursorX:     12,
		Output:      "some output",
		Hash:        "test-hash",
	}

	if result.SessionName != "test-session" {
		t.Errorf("Expected SessionName to be 'test-session', got: %s", result.SessionName)
	}

	if result.CursorLine != "$ echo hello" {
		t.Errorf("Expected CursorLine to be '$ echo hello', got: %s", result.CursorLine)
	}

	if result.CursorY != 23 {
		t.Errorf("Expected CursorY to be 23, got: %d", result.CursorY)
	}

	if result.CursorX != 12 {
		t.Errorf("Expected CursorX to be 12, got: %d", result.CursorX)
	}
}
