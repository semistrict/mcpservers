package tmuxmcp

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBashTool_Simple(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
		asError  bool
	}{
		{
			name:     "Simple command",
			command:  "echo 'hello world'",
			expected: "hello world",
			asError:  false,
		},
		{
			name:     "Command with special characters",
			command:  "echo 'hello \"world\" with \\backslashes'",
			expected: `hello "world" with \backslashes`,
			asError:  false,
		},
		{
			name:     "Command that fails",
			command:  "false", // command that always fails
			expected: "exit code: 1",
			asError:  true,
		},
		{
			name:     "Command with timeout",
			command:  "sleep 10", // command that runs longer than timeout
			expected: "timed out",
			asError:  true,
		},
		{
			name:     "Empty command",
			command:  "", // empty command
			expected: "command is required",
			asError:  true,
		},
		{
			name:     "Command with variables",
			command:  "echo $TEST_VAR",
			expected: "TEST_VAR",
			asError:  true,
		},
		{
			name:     "Command with stderr",
			command:  "echo 'error message' >&2",
			expected: "error message",
			asError:  false, // should not be an error, just captured output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Run subtests in parallel

			tool := NewBashTool()
			tool.Prefix = "test"
			tool.Command = tt.command
			tool.WorkingDirectory = "/tmp"
			tool.Timeout = 2

			if tt.asError {
				errMsg := runErr(t, tool)
				assert.Contains(t, errMsg, tt.expected)
			} else {
				result := run(t, tool)
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}

func TestBashTool_Handle_DefaultTimeout(t *testing.T) {
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "echo test"
	tool.WorkingDirectory = "/tmp"
	tool.Timeout = 2 // Override default for testing
	result := run(t, tool)

	assert.Contains(t, result, "test")
}

func TestBashTool_Handle_ComplexOutput(t *testing.T) {
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "echo 'line1'; echo 'line2' >&2; echo 'line3'" // mixed stdout/stderr
	tool.WorkingDirectory = "/tmp"
	tool.Timeout = 2
	result := run(t, tool)

	// Should capture both stdout and stderr due to 2>&1 | tee
	assert.Contains(t, result, "line1")
	assert.Contains(t, result, "line2") // from stderr
	assert.Contains(t, result, "line3")
}

func TestBashTool_Handle_SpecialCharacters(t *testing.T) {
	// Test with a string that has special characters but no variables to expand
	specialString := `hello "world" with 'quotes' and \backslashes`
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = fmt.Sprintf("echo %s", strconv.Quote(specialString))
	tool.WorkingDirectory = "/tmp"
	tool.Timeout = 2
	result := run(t, tool)

	// Check that quotes and backslashes are preserved
	assert.Contains(t, result, `"world"`)
	assert.Contains(t, result, `'quotes'`)
	assert.Contains(t, result, `\backslashes`)
}

func TestBashTool_Handle_ContextCancellation(t *testing.T) {
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "sleep 2"
	tool.WorkingDirectory = "/tmp"
	tool.Timeout = 5

	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(500*time.Millisecond))
	defer cancel()

	result, err := tool.Handle(ctx)

	if err == nil {
		t.Fatalf("Expected context cancellation error, got result: %v", result)
	}

	if !assert.Contains(t, err.Error(), "timed out") {
		return
	}
}

func TestBashTool_Handle_OutputLimitingShort(t *testing.T) {
	// Test with output less than 50 testLines - should show all output
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "for i in {1..10}; do echo \"Line $i\"; done"
	tool.WorkingDirectory = "/tmp"
	tool.Timeout = 5
	result := run(t, tool)

	// Should contain all testLines without truncation
	for i := 1; i <= 10; i++ {
		expectedLine := fmt.Sprintf("Line %d", i)
		assert.Contains(t, result, expectedLine)
	}

	assert.NotContains(t, result, "Output truncated")
	assert.NotContains(t, result, "available")
}

func TestBashTool_Handle_WorkingDirectory(t *testing.T) {
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "pwd" // Print working directory
	tool.WorkingDirectory = "/tmp"
	tool.Timeout = 2
	result := run(t, tool)

	assert.Contains(t, result, "[1]: /tmp")
}

func TestBashTool_Handle_WorkingDirectory_Default(t *testing.T) {
	// Test that working directory defaults to current directory
	cwd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current working directory")

	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "pwd" // Print working directory
	tool.Timeout = 2
	// WorkingDirectory is intentionally not set, should default to current dir
	result := run(t, tool)

	assert.Contains(t, result, fmt.Sprintf("[1]: %s", cwd))
}

func TestBashTool_Handle_Environment(t *testing.T) {
	// Test that environment variables are properly set
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "echo \"VAR1=$TEST_VAR1 VAR2=$TEST_VAR2\""
	tool.WorkingDirectory = "/tmp"
	tool.Environment = []string{
		"TEST_VAR1=value1",
		"TEST_VAR2=hello world",
	}
	tool.Timeout = 2
	result := run(t, tool)

	assert.Contains(t, result, "[1]: VAR1=value1 VAR2=hello world")
}

func TestBashTool_Handle_Environment_SpecialChars(t *testing.T) {
	// Test environment variables with special characters
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "echo \"VAR=$TEST_VAR\""
	tool.WorkingDirectory = "/tmp"
	tool.Environment = []string{
		`TEST_VAR=special$chars\"with'quotes`,
	}
	tool.Timeout = 2
	result := run(t, tool)

	assert.Contains(t, result, `[1]: VAR=special$chars\"with'quotes`)
}

func TestBashTool_Handle_Environment_Empty(t *testing.T) {
	// Test that empty/nil environment map works fine
	tool := NewBashTool()
	tool.Prefix = "test"
	tool.Command = "echo test"
	tool.WorkingDirectory = "/tmp"
	tool.Environment = nil
	tool.Timeout = 2
	result := run(t, tool)

	assert.Contains(t, result, "[1]: test")
}

func TestBashTool_Handle_Background(t *testing.T) {
	tool := NewBashTool()
	tool.Command = "sleep 5"
	tool.Background = true
	tool.Timeout = 2 // This timeout should be ignored

	errMsg := runErr(t, tool)
	assert.Contains(t, errMsg, "timed out waiting for command in session")
}

func TestBashTool_filtering(t *testing.T) {
	tests := []struct {
		name        string
		contains    []string
		notContains []string
		BashTool
	}{
		{
			name:        "no filters - should show all testLines",
			contains:    []string{"line 1", "line 50", "line 100"},
			notContains: nil,
			BashTool:    BashTool{},
		},
		{
			name:        "head filter - first 10 testLines",
			contains:    []string{"line 1", "line 5", "line 10"},
			notContains: []string{"line 11", "line 50"},
			BashTool:    BashTool{LineBudget: 20},
		},
		{
			name:        "tail filter - last 10 testLines",
			contains:    []string{"line 91", "line 95", "line 100"},
			notContains: []string{"line 50", "line 90"},
			BashTool:    BashTool{LineBudget: 20},
		},
		{
			name:        "grep filter - testLines containing '5'",
			contains:    []string{"line 5", "line 15", "line 50", "line 57", "line 95"},
			notContains: []string{"line 1", "line 2", "line 100"},
			BashTool:    BashTool{Grep: "5", LineBudget: 20},
		},
		{
			name:        "combined grep and head - testLines containing '1' (first 3 results)",
			contains:    []string{"line 1", "line 10", "line 91"},
			notContains: []string{"line 12", "line 13", "line 21"},
			BashTool:    BashTool{Grep: "1", LineBudget: 4},
		},
		{
			name:        "combined grep and tail - testLines containing '9' (last 5 results)",
			contains:    []string{"line 9", "line 99"},
			notContains: []string{"line 29", "line 23", "line 89"},
			BashTool:    BashTool{Grep: "9", LineBudget: 3},
		},
		{
			name:        "grep exclude filter - exclude testLines containing '5'",
			contains:    []string{"line 1", "line 100"},
			notContains: []string{"line 5", "line 15", "line 50", "line 95"},
			BashTool:    BashTool{GrepExclude: "5", LineBudget: 2},
		},
		{
			name:        "grep exclude filter - exclude testLines containing '5' but with high budget",
			contains:    []string{"line 4"},
			notContains: []string{"line 5", "line 95"},
			BashTool:    BashTool{GrepExclude: "5", LineBudget: 100},
		},
		{
			name:        "combined grep and grep exclude - testLines with '1' but not '5'",
			contains:    []string{"line 1", "line 10", "line 11", "line 12", "line 13", "line 14", "line 16", "line 17", "line 18", "line 19"},
			notContains: []string{"line 15", "line 51"},
			BashTool:    BashTool{Grep: "1", GrepExclude: "5", LineBudget: 20},
		},
		{
			name:        "complex regex - testLines ending with 0",
			contains:    []string{"line 10", "line 20", "line 30", "line 40", "line 50", "line 60", "line 70", "line 80", "line 90", "line 100"},
			notContains: []string{"line 1", "line 11", "line 21", "line 99"},
			BashTool:    BashTool{Grep: "0$", LineBudget: 10},
		},
		{
			name:        "complex regex - testLines with exactly 2 digits",
			contains:    []string{"line 10", "line 11", "line 50", "line 99"},
			notContains: []string{"line 1", "line 2", "line 100"},
			BashTool:    BashTool{Grep: "line \\d{2}$", LineBudget: 90},
		},
		{
			name:        "complex regex exclude - exclude testLines with double digits",
			contains:    []string{"line 1", "line 2", "line 3", "line 4", "line 5", "line 6", "line 7", "line 8", "line 9", "line 10", "line 20", "line 30"},
			notContains: []string{"line 11", "line 22", "line 33", "line 44", "line 55", "line 66", "line 77", "line 88", "line 99"},
			BashTool:    BashTool{GrepExclude: "(11|22|33|44|55|66|77|88|99)", LineBudget: 100},
		},
		{
			name:        "complex regex combination - testLines with 1 or 2, excluding those ending in 5",
			contains:    []string{"line 1", "line 2", "line 10", "line 11", "line 12", "line 13", "line 14", "line 16"},
			notContains: []string{"line 15", "line 25", "line 3", "line 4"},
			BashTool:    BashTool{Grep: "[12]", GrepExclude: "5$", LineBudget: 16},
		},
		{
			name:        "complex regex combination - testLines with 1 or 2, excluding those ending in 5 - with context budget",
			contains:    []string{"line 1", "line 2", "line 10", "line 11", "line 12", "line 13", "line 14", "line 16"},
			notContains: []string{"line 15", "line 25", "line 3", "line 4"},
			BashTool:    BashTool{Grep: "[12]", GrepExclude: "5$", LineBudget: 16},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.Command = "false"
			test.WorkingDirectory = "/tmp"
			assert.NoError(t, test.validateArgs())
			resultLines := test.filter(testLines(100))
			var result strings.Builder
			test.displayLines(&result, resultLines)
			for _, contains := range test.contains {
				assert.Contains(t, result.String(), contains+"\n")
			}
			for _, notContains := range test.notContains {
				assert.NotContains(t, result.String(), notContains+"\n")
			}
		})
	}
}

func testLines(n int) <-chan Line {
	lines := make(chan Line)
	go func() {
		for i := 0; i < n; i++ {
			lines <- Line{Number: i + 1, Content: fmt.Sprintf("line %d", i+1), SelectedForOutput: true}
		}
		close(lines)
	}()
	return lines
}

func run(t *testing.T, bc *BashTool) string {
	result, err := bc.Handle(t.Context())
	if assert.NoError(t, err) {
		return result.(string)
	} else {
		return ""
	}
}

func runErr(t *testing.T, bc *BashTool) string {
	output, err := bc.Handle(t.Context())
	assert.Error(t, err, "expected error, got", output)
	return err.Error()
}

func TestBashTool_BackgroundMode(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		timeout     float64
		contains    []string
		notContains []string
	}{
		{
			name:    "Background mode with continuous output",
			command: `for i in {1..20}; do echo "Iteration $i"; sleep 0.05; done`,
			timeout: 1.5,
			contains: []string{
				"Iteration 1",
				"Iteration", // Should capture multiple iterations
				"Hint: Command is still running",
				"You can read the output file directly",
			},
			notContains: []string{
				"exit code", // Should not have exit code in background mode
			},
		},
		{
			name:    "Background mode with stable output",
			command: "echo 'Initial output'; sleep 2; echo 'This should not appear'",
			timeout: 1,
			contains: []string{
				"Initial output",
				"Hint: Command is still running",
			},
			notContains: []string{
				"This should not appear", // Should return before this is printed
			},
		},
		{
			name:    "Background mode with error output",
			command: "echo 'stdout'; echo 'stderr' >&2; sleep 5",
			timeout: 1,
			contains: []string{
				"stdout",
				"stderr", // Should capture both stdout and stderr
				"Hint: Command is still running",
			},
			notContains: []string{
				"exit code",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := &BashTool{
				Prefix:           "test-bg",
				Command:          tt.command,
				WorkingDirectory: "/tmp",
				Timeout:          tt.timeout,
				Background:       true,
				LineBudget:       100,
			}

			ctx := context.Background()
			result, err := tool.Handle(ctx)

			// Check if we got an error or result
			var resultStr string
			if err != nil {
				resultStr = err.Error()
			} else {
				resultStr = result.(string)
			}

			for _, expected := range tt.contains {
				assert.Contains(t, resultStr, expected)
			}

			for _, unexpected := range tt.notContains {
				assert.NotContains(t, resultStr, unexpected)
			}

			// Verify session is still running
			sessionExists := sessionExists(ctx, tool.sessionName)
			assert.True(t, sessionExists, "Session should still be running in background mode")

			// Clean up
			if sessionExists {
				runTmuxCommand(ctx, "kill-session", "-t", tool.sessionName)
			}
		})
	}
}

func TestBashTool_BackgroundMode_QuickExit(t *testing.T) {
	// Test that background mode handles commands that exit quickly
	tool := &BashTool{
		Prefix:           "test-bg-quick",
		Command:          "echo 'Quick exit'; exit 0",
		WorkingDirectory: "/tmp",
		Timeout:          2,
		Background:       true,
		LineBudget:       100,
	}

	ctx := context.Background()
	result, err := tool.Handle(ctx)

	// Check if we got an error or result
	var resultStr string
	if err != nil {
		resultStr = err.Error()
	} else {
		resultStr = result.(string)
	}

	assert.Contains(t, resultStr, "Quick exit")

	// Even if command exited, we should have captured output
	time.Sleep(100 * time.Millisecond)
	_ = sessionExists(ctx, tool.sessionName)
	// Session may or may not exist depending on timing, but we should have output
}

func TestBashTool_BackgroundMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		timeout      float64
		contains     []string
		notContains  []string
		expectError  bool
		checkSession bool // whether to check if session is still running
	}{
		{
			name:    "Command returns immediately with success",
			command: "echo 'Immediate success'; exit 0",
			timeout: 1,
			contains: []string{
				"Immediate success",
			},
			notContains: []string{
				"exit code", // Background mode doesn't check exit codes
			},
			expectError:  false,
			checkSession: false, // Session may have already exited
		},
		{
			name:    "Command returns immediately with error",
			command: "echo 'Error output' >&2; exit 1",
			timeout: 1,
			contains: []string{
				"Error output",
			},
			notContains: []string{
				"exit code", // Background mode doesn't check exit codes
			},
			expectError:  false, // Background mode doesn't report exit codes as errors
			checkSession: false,
		},
		{
			name:    "Command that never returns",
			command: "echo 'Starting infinite loop'; while true; do sleep 1; done",
			timeout: 1,
			contains: []string{
				"Starting infinite loop",
				"Hint: Command is still running",
				"You can read the output file directly",
			},
			notContains:  []string{},
			expectError:  false,
			checkSession: true, // Should still be running
		},
		{
			name:    "Command with no output",
			command: "sleep 10",
			timeout: 1,
			contains: []string{
				"completed successfully but produced no output",
			},
			notContains:  []string{},
			expectError:  false,
			checkSession: true,
		},
		{
			name:    "Command that produces output then exits with error",
			command: "echo 'Starting...'; sleep 0.5; echo 'Error!' >&2; exit 42",
			timeout: 1,
			contains: []string{
				"Starting...",
				"Error!",
			},
			notContains: []string{
				"exit code", // Background mode doesn't check exit codes
			},
			expectError:  false,
			checkSession: false,
		},
		{
			name:    "Command with rapid output then stability",
			command: "for i in {1..5}; do echo \"Line $i\"; done; sleep 10",
			timeout: 2,
			contains: []string{
				"Line 1",
				"Line 2",
				"Line 3",
				"Line 4",
				"Line 5",
			},
			notContains:  []string{},
			expectError:  false,
			checkSession: true,
		},
		{
			name:    "Command that fails to start",
			command: "/nonexistent/command",
			timeout: 1,
			contains: []string{
				"No such file or directory", // Actual shell error message
			},
			notContains:  []string{},
			expectError:  false, // Error is captured in output, not returned
			checkSession: false,
		},
		{
			name:    "Command with very long lines",
			command: "echo 'Start'; printf 'A%.0s' {1..3000}; echo 'End'",
			timeout: 1,
			contains: []string{
				"Start",
				"AAAA", // Should contain the A's
				"End",
			},
			notContains:  []string{},
			expectError:  false,
			checkSession: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := &BashTool{
				Prefix:           "test-bg-edge",
				Command:          tt.command,
				WorkingDirectory: "/tmp",
				Timeout:          tt.timeout,
				Background:       true,
				LineBudget:       100,
			}

			ctx := context.Background()
			result, err := tool.Handle(ctx)

			// Check error expectation
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Get result string
			var resultStr string
			if err != nil {
				resultStr = err.Error()
			} else if result != nil {
				resultStr = result.(string)
			}

			// Check expected content
			for _, expected := range tt.contains {
				assert.Contains(t, resultStr, expected, "Expected to find: %s", expected)
			}

			for _, unexpected := range tt.notContains {
				assert.NotContains(t, resultStr, unexpected, "Should not contain: %s", unexpected)
			}

			// Check session status if needed
			if tt.checkSession {
				time.Sleep(100 * time.Millisecond) // Give session time to stabilize
				exists := sessionExists(ctx, tool.sessionName)
				assert.True(t, exists, "Session should still be running for: %s", tt.name)
			}

			// Clean up sessions
			if tool.sessionName != "" {
				// Try to kill session, ignore errors (it may have already exited)
				runTmuxCommand(ctx, "kill-session", "-t", tool.sessionName)
			}
		})
	}
}
