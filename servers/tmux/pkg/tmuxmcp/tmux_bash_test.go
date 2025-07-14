package tmuxmcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBashTool_Handle_Success(t *testing.T) {
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "echo 'hello world'",
		Timeout: 2,
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

	if !strings.Contains(resultStr, "Command completed successfully (exit code 0)") {
		t.Errorf("Expected success message, got: %s", resultStr)
	}

	if !strings.Contains(resultStr, "hello world") {
		t.Errorf("Expected output to contain 'hello world', got: %s", resultStr)
	}
}

func TestBashTool_Handle_Failure(t *testing.T) {
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "false", // command that always fails
		Timeout: 2,
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

	if !strings.Contains(resultStr, "Command failed (exit code 1)") {
		t.Errorf("Expected failure message, got: %s", resultStr)
	}
}

func TestBashTool_Handle_Timeout(t *testing.T) {
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "sleep 3", // command that runs longer than timeout
		Timeout: 0.5,       // 0.5 second timeout
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err == nil {
		t.Fatalf("Expected timeout error, got result: %v", result)
	}

	if !strings.Contains(err.Error(), "command timed out") {
		t.Errorf("Expected timeout error message, got: %s", err.Error())
	}

	if !strings.Contains(err.Error(), "still running in session") {
		t.Errorf("Expected session name in timeout error, got: %s", err.Error())
	}

	// Clean up the session that's still running
	// Extract session name from error message
	parts := strings.Split(err.Error(), "still running in session: ")
	if len(parts) == 2 {
		sessionName := parts[1]
		killSession(sessionName) // Clean up
	}
}

func TestBashTool_Handle_EmptyCommand(t *testing.T) {
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "", // empty command
		Timeout: 5,
	}

	ctx := t.Context()
	result, err := tool.Handle(ctx)

	if err == nil {
		t.Fatalf("Expected error for empty command, got result: %v", result)
	}

	if !strings.Contains(err.Error(), "command is required") {
		t.Errorf("Expected 'command is required' error, got: %s", err.Error())
	}
}

func TestBashTool_Handle_DefaultTimeout(t *testing.T) {
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "echo test",
		Timeout: 2, // Override default for testing
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

	if !strings.Contains(resultStr, "Command completed successfully") {
		t.Errorf("Expected success message, got: %s", resultStr)
	}
}

func TestBashTool_Handle_ComplexOutput(t *testing.T) {
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "echo 'line1'; echo 'line2' >&2; echo 'line3'", // mixed stdout/stderr
		Timeout: 2,
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

	// Should capture both stdout and stderr due to 2>&1 | tee
	if !strings.Contains(resultStr, "line1") {
		t.Errorf("Expected output to contain 'line1', got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "line2") {
		t.Errorf("Expected output to contain 'line2' (from stderr), got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "line3") {
		t.Errorf("Expected output to contain 'line3', got: %s", resultStr)
	}
}

func TestBashTool_Handle_SpecialCharacters(t *testing.T) {
	// Test with a string that has special characters but no variables to expand
	specialString := `hello "world" with 'quotes' and \backslashes`
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: fmt.Sprintf("echo %s", strconv.Quote(specialString)),
		Timeout: 2,
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

	// Check that quotes and backslashes are preserved
	if !strings.Contains(resultStr, `"world"`) {
		t.Errorf("Expected output to contain double quotes, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, `'quotes'`) {
		t.Errorf("Expected output to contain single quotes, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, `\backslashes`) {
		t.Errorf("Expected output to contain backslashes, got: %s", resultStr)
	}
}

func TestBashTool_Handle_ContextCancellation(t *testing.T) {
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "sleep 2",
		Timeout: 5,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	result, err := tool.Handle(ctx)

	if err == nil {
		t.Fatalf("Expected context cancellation error, got result: %v", result)
	}

	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("Expected context cancellation error, got: %s", err.Error())
	}

	// Clean up the session that might still be running
	if strings.Contains(err.Error(), "still running in session: ") {
		parts := strings.Split(err.Error(), "still running in session: ")
		if len(parts) == 2 {
			sessionName := parts[1]
			killSession(sessionName)
		}
	}
}

func TestBashTool_Handle_OutputLimitingShort(t *testing.T) {
	// Test with output less than 50 lines - should show all output
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "for i in {1..10}; do echo \"Line $i\"; done",
		Timeout: 5,
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

	// Should contain all lines without truncation
	for i := 1; i <= 10; i++ {
		expectedLine := fmt.Sprintf("Line %d", i)
		if !strings.Contains(resultStr, expectedLine) {
			t.Errorf("Expected output to contain '%s', got: %s", expectedLine, resultStr)
		}
	}

	// Should NOT contain truncation message
	if strings.Contains(resultStr, "Output truncated") {
		t.Errorf("Expected no truncation for short output, but got truncation message")
	}

	if strings.Contains(resultStr, "available in:") {
		t.Errorf("Expected no temp file reference for short output, but got temp file message")
	}
}

func TestBashTool_Handle_OutputLimitingLong(t *testing.T) {
	// Test with output more than 50 lines - should truncate and provide temp file
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "for i in {1..100}; do echo \"Line $i\"; done",
		Timeout: 10,
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

	// Should contain first few lines
	if !strings.Contains(resultStr, "Line 1") {
		t.Errorf("Expected output to contain 'Line 1', got: %s", resultStr)
	}

	// Should contain truncation message
	if !strings.Contains(resultStr, "Output truncated to first 50 non-empty lines") {
		t.Errorf("Expected truncation message, got: %s", resultStr)
	}

	// Should contain temp file reference
	if !strings.Contains(resultStr, "available in:") {
		t.Errorf("Expected temp file reference, got: %s", resultStr)
	}

	// Should contain line count
	if !strings.Contains(resultStr, "100 non-empty lines") {
		t.Errorf("Expected total line count in message, got: %s", resultStr)
	}

	// Should NOT contain later lines (like Line 90)
	if strings.Contains(resultStr, "Line 90") {
		t.Errorf("Expected output to be truncated and not contain 'Line 90', got: %s", resultStr)
	}
}

func TestBashTool_Handle_OutputLimitingExactly50Lines(t *testing.T) {
	// Test with exactly 50 lines - should show all without truncation
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "for i in {1..50}; do echo \"Line $i\"; done",
		Timeout: 5,
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

	// Should contain all lines
	if !strings.Contains(resultStr, "Line 1") {
		t.Errorf("Expected output to contain 'Line 1', got: %s", resultStr)
	}

	if !strings.Contains(resultStr, "Line 50") {
		t.Errorf("Expected output to contain 'Line 50', got: %s", resultStr)
	}

	// Should NOT contain truncation message since it's exactly 50 lines
	if strings.Contains(resultStr, "Output truncated") {
		t.Errorf("Expected no truncation for exactly 50 lines, but got truncation message")
	}
}

func TestBashTool_Handle_OutputLimiting51Lines(t *testing.T) {
	// Test with 51 lines - should truncate to 50
	tool := &BashTool{
		SessionTool: SessionTool{
			Prefix: "test",
		},
		Command: "for i in {1..51}; do echo \"Line $i\"; done",
		Timeout: 5,
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

	// Should contain first 50 lines
	if !strings.Contains(resultStr, "Line 1") {
		t.Errorf("Expected output to contain 'Line 1', got: %s", resultStr)
	}

	if !strings.Contains(resultStr, "Line 50") {
		t.Errorf("Expected output to contain 'Line 50', got: %s", resultStr)
	}

	// Should NOT contain the 51st line
	if strings.Contains(resultStr, "Line 51") {
		t.Errorf("Expected output to be truncated and not contain 'Line 51', got: %s", resultStr)
	}

	// Should contain truncation message
	if !strings.Contains(resultStr, "Output truncated to first 50 non-empty lines") {
		t.Errorf("Expected truncation message, got: %s", resultStr)
	}

	// Should mention 51 lines total
	if !strings.Contains(resultStr, "51 non-empty lines") {
		t.Errorf("Expected total line count (51) in message, got: %s", resultStr)
	}
}
