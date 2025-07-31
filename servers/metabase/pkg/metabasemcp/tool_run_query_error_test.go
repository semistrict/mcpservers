package metabasemcp

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunQueryTool_ErrorHandling(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err)

	tests := []struct {
		name          string
		query         string
		expectedError string
	}{
		{
			name:          "Invalid table",
			query:         "SELECT * FROM non_existent_table",
			expectedError: "Table \"NON_EXISTENT_TABLE\" not found",
		},
		{
			name:          "Syntax error",
			query:         "SELECT * FORM PEOPLE", // Typo: FORM instead of FROM
			expectedError: "Syntax error",
		},
		{
			name:          "Invalid column",
			query:         "SELECT non_existent_column FROM PEOPLE",
			expectedError: "Column \"NON_EXISTENT_COLUMN\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &RunQueryTool{
				DatabaseID: 1,
				Query:      tt.query,
			}

			ctx := context.Background()
			result, err := tool.Handle(ctx)

			// Should return an error
			require.Error(t, err)
			assert.Nil(t, result)

			// Error should contain the expected message
			assert.Contains(t, err.Error(), "Query failed:")
			assert.Contains(t, err.Error(), tt.expectedError)

			// Error should include additional details
			errorStr := err.Error()
			if strings.Contains(errorStr, "Error type:") {
				t.Logf("Error includes type information")
			}
			if strings.Contains(errorStr, "SQL State:") {
				t.Logf("Error includes SQL state")
			}
			if strings.Contains(errorStr, "Exception class:") {
				t.Logf("Error includes exception class")
			}

			t.Logf("Full error: %v", err)
		})
	}
}

func TestRunQueryTool_SuccessfulQuery(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err)

	tool := &RunQueryTool{
		DatabaseID: 1,
		Query:      "SELECT * FROM PEOPLE LIMIT 5",
	}

	ctx := context.Background()
	result, err := tool.Handle(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Result should be markdown format
	markdown, ok := result.(string)
	require.True(t, ok, "Result should be a string")
	assert.Contains(t, markdown, "|") // Markdown tables contain pipes
	t.Logf("Query result preview: %s", strings.Split(markdown, "\n")[0])
}

func TestRunQueryTool_Validation(t *testing.T) {
	tests := []struct {
		name          string
		tool          *RunQueryTool
		expectedError string
	}{
		{
			name: "Missing database ID",
			tool: &RunQueryTool{
				Query: "SELECT 1",
			},
			expectedError: "database_id is required",
		},
		{
			name: "Missing query",
			tool: &RunQueryTool{
				DatabaseID: 1,
			},
			expectedError: "query is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := tt.tool.Handle(ctx)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}
