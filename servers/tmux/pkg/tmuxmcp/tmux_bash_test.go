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

			tool := &BashTool{
				Prefix:           "test",
				Command:          tt.command,
				WorkingDirectory: "/tmp",
				Timeout:          2,
			}

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
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "echo test",
		WorkingDirectory: "/tmp",
		Timeout:          2, // Override default for testing
	})

	assert.Contains(t, result, "test")
}

func TestBashTool_Handle_ComplexOutput(t *testing.T) {
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "echo 'line1'; echo 'line2' >&2; echo 'line3'", // mixed stdout/stderr
		WorkingDirectory: "/tmp",
		Timeout:          2,
	})

	// Should capture both stdout and stderr due to 2>&1 | tee
	assert.Contains(t, result, "line1")
	assert.Contains(t, result, "line2") // from stderr
	assert.Contains(t, result, "line3")
}

func TestBashTool_Handle_SpecialCharacters(t *testing.T) {
	// Test with a string that has special characters but no variables to expand
	specialString := `hello "world" with 'quotes' and \backslashes`
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          fmt.Sprintf("echo %s", strconv.Quote(specialString)),
		WorkingDirectory: "/tmp",
		Timeout:          2,
	})

	// Check that quotes and backslashes are preserved
	assert.Contains(t, result, `"world"`)
	assert.Contains(t, result, `'quotes'`)
	assert.Contains(t, result, `\backslashes`)
}

func TestBashTool_Handle_ContextCancellation(t *testing.T) {
	tool := &BashTool{
		Prefix:           "test",
		Command:          "sleep 2",
		WorkingDirectory: "/tmp",
		Timeout:          5,
	}

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
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "for i in {1..10}; do echo \"Line $i\"; done",
		WorkingDirectory: "/tmp",
		Timeout:          5,
	})

	// Should contain all testLines without truncation
	for i := 1; i <= 10; i++ {
		expectedLine := fmt.Sprintf("Line %d", i)
		assert.Contains(t, result, expectedLine)
	}

	assert.NotContains(t, result, "Output truncated")
	assert.NotContains(t, result, "available")
}

func TestBashTool_Handle_WorkingDirectory(t *testing.T) {
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "pwd", // Print working directory
		WorkingDirectory: "/tmp",
		Timeout:          2,
	})

	assert.Contains(t, result, "[1]: /tmp")
}

func TestBashTool_Handle_WorkingDirectory_Default(t *testing.T) {
	// Test that working directory defaults to current directory
	cwd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current working directory")

	result := run(t, &BashTool{
		Prefix:  "test",
		Command: "pwd", // Print working directory
		Timeout: 2,
		// WorkingDirectory is intentionally not set
	})

	assert.Contains(t, result, fmt.Sprintf("[1]: %s", cwd))
}

func TestBashTool_Handle_Environment(t *testing.T) {
	// Test that environment variables are properly set
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "echo \"VAR1=$TEST_VAR1 VAR2=$TEST_VAR2\"",
		WorkingDirectory: "/tmp",
		Environment: []string{
			"TEST_VAR1=value1",
			"TEST_VAR2=hello world",
		},
		Timeout: 2,
	})

	assert.Contains(t, result, "[1]: VAR1=value1 VAR2=hello world")
}

func TestBashTool_Handle_Environment_SpecialChars(t *testing.T) {
	// Test environment variables with special characters
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "echo \"VAR=$TEST_VAR\"",
		WorkingDirectory: "/tmp",
		Environment: []string{
			`TEST_VAR=special$chars\"with'quotes`,
		},
		Timeout: 2,
	})

	assert.Contains(t, result, `[1]: VAR=special$chars\"with'quotes`)
}

func TestBashTool_Handle_Environment_Empty(t *testing.T) {
	// Test that empty/nil environment map works fine
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "echo test",
		WorkingDirectory: "/tmp",
		Environment:      nil,
		Timeout:          2,
	})

	assert.Contains(t, result, "[1]: test")
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
