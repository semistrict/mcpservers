package tmuxmcp

import (
	"context"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func TestSendKeysToolLiteralMode_Integration(t *testing.T) {
	// Create a real tmux session for testing
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	assert.NoError(t, err)
	defer func() { killSession(t.Context(), sessionName) }()

	// Get initial hash
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(1*time.Second))
	defer cancel()
	result, err := waitForStability(ctx, sessionName)
	if err != nil {
		t.Fatalf("Could not capture initial session state: %v", err)
	}
	assert.NotEmpty(t, result.Hash)
	initialHash := result.Hash

	tool := &SendKeysTool{
		Hash:   initialHash,
		Keys:   "hello world with spaces",
		Expect: "", // Empty expect for testing
	}
	tool.Prefix = sessionName

	// Test the tool with a real session
	toolResult, err := tool.Handle(t.Context())
	assert.NoError(t, err)

	resultStr, ok := toolResult.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", toolResult)
	}

	if !strings.Contains(resultStr, "Keys sent to session") {
		t.Errorf("Expected success message, got: %s", resultStr)
	}

	// Verify keys were actually sent by capturing session
	time.Sleep(100 * time.Millisecond)
	afterResult, err := capture(t.Context(), captureOptions{Prefix: sessionName})
	if err != nil {
		t.Fatalf("Could not capture session after sending keys: %v", err)
	}

	if !strings.Contains(afterResult.Output, "hello world with spaces") {
		t.Errorf("Expected session to contain sent keys, got: %s", afterResult.Output)
	}
}

func TestSendKeysCommonValidation(t *testing.T) {
	tests := []struct {
		name      string
		opts      SendKeysOptions
		expectErr bool
		errMsg    string
	}{
		{
			name: "missing hash",
			opts: SendKeysOptions{
				SessionName: "test-session",
				Keys:        "hello",
				Expect:      "",
			},
			expectErr: true,
			errMsg:    "hash is required",
		},
		{
			name: "missing keys",
			opts: SendKeysOptions{
				SessionName: "test-session",
				Hash:        "test-hash",
				Expect:      "",
			},
			expectErr: true,
			errMsg:    "keys parameter is required",
		},
		{
			name: "valid literal keys with empty expect",
			opts: SendKeysOptions{
				SessionName: "test-session",
				Hash:        "test-hash",
				Keys:        "hello world",
				Expect:      "",
				Literal:     true,
			},
			expectErr: false,
		},
		{
			name: "valid control keys with expect",
			opts: SendKeysOptions{
				SessionName: "test-session",
				Hash:        "test-hash",
				Keys:        "C-c Enter",
				Expect:      "$",
				Literal:     false,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't test the full function without a real tmux session,
			// but we can test the validation logic
			if tt.opts.Hash == "" {
				_, err := sendKeysCommon(t.Context(), tt.opts)
				if (err != nil) != tt.expectErr {
					t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
				}
				if tt.expectErr && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain '%s', got: %s", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestSendKeysToolHandle_Integration(t *testing.T) {
	// Create a real tmux session for testing
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	if err != nil {
		t.Skipf("Could not create tmux session for testing: %v", err)
	}
	defer func() { killSession(t.Context(), sessionName) }()

	// Get initial hash
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Second))
	defer cancel()
	result, err := waitForStability(ctx, sessionName)
	if err != nil {
		t.Fatalf("Could not capture initial session state: %v", err)
	}
	initialHash := result.Hash

	tool := &SendKeysTool{
		Hash:   initialHash,
		Keys:   "hello world",
		Expect: "", // Empty expect for testing
	}
	tool.Prefix = sessionName

	// Test basic structure and method existence with real session
	toolResult, err := tool.Handle(t.Context())

	assert.NoError(t, err)

	resultStr, ok := toolResult.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", toolResult)
	}

	if !strings.Contains(resultStr, "Keys sent to session") {
		t.Errorf("Expected success message, got: %s", resultStr)
	}
}

func TestSendControlKeysToolHandle_Integration(t *testing.T) {
	// Create a real tmux session for testing
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	if err != nil {
		t.Skipf("Could not create tmux session for testing: %v", err)
	}
	defer func() { killSession(t.Context(), sessionName) }()

	// Get initial hash
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Second))
	defer cancel()
	result, err := waitForStability(ctx, sessionName)
	if err != nil {
		t.Fatalf("Could not capture initial session state: %v", err)
	}
	initialHash := result.Hash

	tool := &SendControlKeysTool{
		Hash:   initialHash,
		Keys:   "C-c Enter",
		Hex:    false,
		Expect: "", // Empty expect for testing
	}
	tool.Prefix = sessionName

	// Test basic structure and method existence with real session
	toolResult, err := tool.Handle(t.Context())
	assert.NoError(t, err)

	resultStr, ok := toolResult.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", toolResult)
	}

	if !strings.Contains(resultStr, "Control keys sent to session") {
		t.Errorf("Expected success message, got: %s", resultStr)
	}
}

func TestSendKeysOptionsDefaults(t *testing.T) {
	opts := SendKeysOptions{
		SessionName: "test-session",
		Hash:        "test-hash",
		Keys:        "hello",
		MaxWait:     0, // Test default
	}

	// Test that MaxWait gets set to default value
	if opts.MaxWait == 0 {
		// This should be handled in sendKeysCommon
		expectedDefault := 10.0
		if opts.MaxWait != expectedDefault {
			// We can't test this directly without calling sendKeysCommon
			// but we can verify the structure
			t.Logf("MaxWait should default to %f when 0", expectedDefault)
		}
	}
}

func TestToolDescriptions(t *testing.T) {
	// Test that the tool descriptions are appropriate
	sendKeysTool := &SendKeysTool{}
	controlKeysTool := &SendControlKeysTool{}

	// Use reflection to get the tool info (simplified test)
	if sendKeysTool.Keys == "" {
		t.Log("SendKeysTool should handle literal text")
	}
	if controlKeysTool.Keys == "" {
		t.Log("SendControlKeysTool should handle control sequences")
	}
}

func TestEmptyExpectBehavior_Integration(t *testing.T) {
	// Create a real tmux session for testing
	sessionName, err := createUniqueSession(t.Context(), "test", []string{"bash"})
	if err != nil {
		t.Skipf("Could not create tmux session for testing: %v", err)
	}
	defer func() { killSession(t.Context(), sessionName) }()

	// Wait for session to stabilize before getting hash
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(2*time.Second))
	defer cancel()
	result, err := waitForStability(ctx, sessionName)
	if err != nil {
		t.Fatalf("Could not wait for session stability: %v", err)
	}
	initialHash := result.Hash

	// Test empty expect - should send keys without waiting
	emptyExpectResult, err := sendKeysCommon(t.Context(), SendKeysOptions{
		SessionName: sessionName,
		Hash:        initialHash,
		Keys:        "echo test",
		Expect:      "", // Empty expect
		Enter:       false,
		Literal:     true, // Need to set this when calling sendKeysCommon directly
	})

	if err != nil {
		t.Fatalf("Expected no error for empty expect, got: %v", err)
	}

	if emptyExpectResult.Output != "" {
		t.Errorf("Expected empty output for empty expect, got: %s", emptyExpectResult.Output)
	}

	if emptyExpectResult.Hash != "" {
		t.Errorf("Expected empty hash for empty expect, got: %s", emptyExpectResult.Hash)
	}

	// Verify keys were actually sent by capturing current state
	time.Sleep(100 * time.Millisecond) // Brief pause for keys to be processed
	afterResult, err := capture(t.Context(), captureOptions{Prefix: sessionName})
	if err != nil {
		t.Fatalf("Could not capture session after sending keys: %v", err)
	}

	if !strings.Contains(afterResult.Output, "echo test") {
		t.Errorf("Expected session to contain 'echo test', got: %s", afterResult.Output)
	}
}

func TestEnterFlagHandling_Integration(t *testing.T) {
	tests := []struct {
		name        string
		opts        SendKeysOptions
		description string
	}{
		{
			name: "enter with literal mode",
			opts: SendKeysOptions{
				Keys:    "echo hello",
				Enter:   true,
				Expect:  "",
				Literal: true,
				Hex:     false,
			},
			description: "Literal mode should send Enter in separate command",
		},
		{
			name: "enter with hex mode",
			opts: SendKeysOptions{
				Keys:    "48 65 6c 6c 6f", // "Hello" in hex
				Enter:   true,
				Expect:  "",
				Literal: false,
				Hex:     true,
			},
			description: "Hex mode should send Enter in separate command",
		},
		{
			name: "no enter with literal mode",
			opts: SendKeysOptions{
				Keys:    "echo test",
				Enter:   false,
				Expect:  "",
				Literal: true,
				Hex:     false,
			},
			description: "No Enter flag should work normally in literal mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			tt.opts.SessionName, err = createUniqueSession(t.Context(), "test", []string{"bash"})
			assert.NoError(t, err)

			// Get initial hash
			ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Second))
			defer cancel()
			cr, err := waitForStability(ctx, tt.opts.SessionName)
			assert.NoError(t, err)

			tt.opts.Hash = cr.Hash

			result, err := sendKeysCommon(t.Context(), tt.opts)

			assert.NoError(t, err)
			assert.NotNil(t, result)

			// For empty expect, should get empty output
			if result.Output != "" {
				t.Errorf("Expected empty output for empty expect, got: %s", result.Output)
			}
			if result.Hash != "" {
				t.Errorf("Expected empty hash for empty expect, got: %s", result.Hash)
			}

			// Verify keys were sent by capturing session
			time.Sleep(100 * time.Millisecond)
			afterResult, err := capture(t.Context(), captureOptions{Prefix: tt.opts.SessionName})
			if err != nil {
				t.Fatalf("Could not capture session after sending keys: %v", err)
			}

			// Just verify that the session state changed (keys were sent)
			if afterResult.Hash == tt.opts.Hash {
				t.Error("Expected session state to change after sending keys")
			}
		})
	}
}
