package tmuxmcp

import (
	"context"
	"fmt"
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
	// Test with output less than 50 lines - should show all output
	result := run(t, &BashTool{
		Prefix:           "test",
		Command:          "for i in {1..10}; do echo \"Line $i\"; done",
		WorkingDirectory: "/tmp",
		Timeout:          5,
	})

	// Should contain all lines without truncation
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

func TestBashTool_filtering(t *testing.T) {
	tests := []struct {
		name        string
		contains    []string
		notContains []string
		BashTool
	}{
		{
			name:        "no filters - should show all lines",
			contains:    []string{"line 1", "line 50", "line 100"},
			notContains: nil,
			BashTool:    BashTool{},
		},
		{
			name:        "head filter - first 10 lines",
			contains:    []string{"line 1", "line 5", "line 10"},
			notContains: []string{"line 11", "line 50", "line 100"},
			BashTool:    BashTool{Head: 10},
		},
		{
			name:        "tail filter - last 10 lines",
			contains:    []string{"line 91", "line 95", "line 100"},
			notContains: []string{"line 1", "line 50", "line 90"},
			BashTool:    BashTool{Tail: 10},
		},
		{
			name:        "grep filter - lines containing '5'",
			contains:    []string{"line 5", "line 15", "line 50", "line 95"},
			notContains: []string{"line 1", "line 2", "line 100"},
			BashTool:    BashTool{Grep: "5"},
		},
		{
			name:        "combined grep and head - lines containing '1' (first 3 results)",
			contains:    []string{"line 1", "line 10", "line 11"},
			notContains: []string{"line 12", "line 13", "line 21"},
			BashTool:    BashTool{Head: 3, Grep: "1"},
		},
		{
			name:        "combined grep and tail - lines containing '9' (last 5 results)",
			contains:    []string{"line 95", "line 96", "line 97", "line 98", "line 99"},
			notContains: []string{"line 9", "line 19", "line 89"},
			BashTool:    BashTool{Tail: 5, Grep: "9"},
		},
		{
			name:        "grep exclude filter - exclude lines containing '5'",
			contains:    []string{"line 1", "line 2", "line 100"},
			notContains: []string{"line 5", "line 15", "line 50", "line 95"},
			BashTool:    BashTool{GrepExclude: "5"},
		},
		{
			name:        "combined grep and grep exclude - lines with '1' but not '5'",
			contains:    []string{"line 1", "line 10", "line 11", "line 12", "line 13", "line 14", "line 16", "line 17", "line 18", "line 19"},
			notContains: []string{"line 15", "line 51"},
			BashTool:    BashTool{Grep: "1", GrepExclude: "5"},
		},
		{
			name:        "complex regex - lines ending with 0",
			contains:    []string{"line 10", "line 20", "line 30", "line 40", "line 50", "line 60", "line 70", "line 80", "line 90", "line 100"},
			notContains: []string{"line 1", "line 11", "line 21", "line 99"},
			BashTool:    BashTool{Grep: "0$"},
		},
		{
			name:        "complex regex - lines with exactly 2 digits",
			contains:    []string{"line 10", "line 11", "line 50", "line 99"},
			notContains: []string{"line 1", "line 2", "line 100"},
			BashTool:    BashTool{Grep: "line \\d{2}$"},
		},
		{
			name:        "complex regex exclude - exclude lines with double digits",
			contains:    []string{"line 1", "line 2", "line 3", "line 4", "line 5", "line 6", "line 7", "line 8", "line 9", "line 10", "line 20", "line 30"},
			notContains: []string{"line 11", "line 22", "line 33", "line 44", "line 55", "line 66", "line 77", "line 88", "line 99"},
			BashTool:    BashTool{GrepExclude: "(11|22|33|44|55|66|77|88|99)"},
		},
		{
			name:        "complex regex combination - lines with 1 or 2, excluding those ending in 5",
			contains:    []string{"line 1", "line 2", "line 10", "line 11", "line 12", "line 13", "line 14", "line 16"},
			notContains: []string{"line 15", "line 25", "line 3", "line 4"},
			BashTool:    BashTool{Grep: "[12]", GrepExclude: "5$", Head: 8},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.Command = "false"
			test.WorkingDirectory = "/tmp"
			assert.NoError(t, test.validateArgs())

			l := test.filter(lines(100))
			result := collect(t, l)
			for _, contains := range test.contains {
				assert.Contains(t, result, contains+"\n")
			}
			for _, notContains := range test.notContains {
				assert.NotContains(t, result, notContains+"\n")
			}
		})
	}
}

func collect(t *testing.T, l <-chan Line) string {
	var buf strings.Builder
	for line := range l {
		assert.NoError(t, line.Error)
		fmt.Fprintln(&buf, line.Content)
	}
	return buf.String()
}

func lines(n int) <-chan Line {
	lines := make(chan Line)
	go func() {
		for i := 0; i < n; i++ {
			lines <- Line{i + 1, fmt.Sprintf("line %d", i+1), nil}
		}
		close(lines)
	}()
	return lines
}

func run(t *testing.T, bc *BashTool) string {
	result, err := bc.Handle(t.Context())
	assert.NoError(t, err)
	return result.(string)
}

func runErr(t *testing.T, bc *BashTool) string {
	output, err := bc.Handle(t.Context())
	assert.Error(t, err, "expected error, got", output)
	return err.Error()
}
