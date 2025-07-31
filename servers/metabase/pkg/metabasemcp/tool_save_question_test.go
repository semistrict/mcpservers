package metabasemcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveQuestionTool(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err)

	// Generate unique name with timestamp
	timestamp := time.Now().Format("20060102_150405")
	questionName := "Test Question " + timestamp

	tool := &SaveQuestionTool{
		DatabaseTool: DatabaseTool{
			DatabaseID: 1,
		},
		Name:        questionName,
		Query:       "SELECT COUNT(*) as total_people FROM PEOPLE",
		Description: "Test question created by save_question tool test",
		Display:     "scalar",
	}

	ctx := context.Background()
	result, err := tool.Handle(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check the response
	response, ok := result.(string)
	require.True(t, ok, "Result should be a string")

	// Verify response contains expected information
	assert.Contains(t, response, "Question saved successfully!")
	assert.Contains(t, response, "ID:")
	assert.Contains(t, response, "Name: "+questionName)
	assert.Contains(t, response, "Description: Test question created by save_question tool test")
	assert.Contains(t, response, "Database ID: 1")
	assert.Contains(t, response, "Display Type: scalar")
	assert.Contains(t, response, "Created:")

	t.Logf("Save question response:\n%s", response)
}

func TestSaveQuestionTool_TableDisplay(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err)

	// Generate unique name
	timestamp := time.Now().Format("20060102_150405")
	questionName := "Product Report " + timestamp

	tool := &SaveQuestionTool{
		Name:  questionName,
		Query: "SELECT * FROM PRODUCTS ORDER BY PRICE DESC LIMIT 10",
		DatabaseTool: DatabaseTool{
			DatabaseID: 1,
		},
		// Don't specify display to test default
	}

	ctx := context.Background()
	result, err := tool.Handle(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)

	response, ok := result.(string)
	require.True(t, ok)

	// Should default to table display
	assert.Contains(t, response, "Display Type: table")
	t.Logf("Default display type confirmed: table")
}

func TestSaveQuestionTool_WithCollection(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err)

	// Use collection ID 2 (Examples collection)
	collectionID := 2
	timestamp := time.Now().Format("20060102_150405")
	questionName := "Analytics Report " + timestamp

	tool := &SaveQuestionTool{
		Name:  questionName,
		Query: "SELECT DATE_TRUNC('month', CREATED_AT) as month, COUNT(*) as new_users FROM PEOPLE GROUP BY 1 ORDER BY 1",
		DatabaseTool: DatabaseTool{
			DatabaseID: 1,
		},
		Display:      "line",
		CollectionID: collectionID,
	}

	ctx := context.Background()
	result, err := tool.Handle(ctx)

	// Note: This might fail if collection 1 doesn't exist or user doesn't have permission
	if err != nil {
		t.Fatalf("Failed to save question to collection: %v. Ensure collection 1 exists and user has permission.", err)
	}

	require.NotNil(t, result)
	response, ok := result.(string)
	require.True(t, ok)

	assert.Contains(t, response, "Collection ID: 2")
	t.Logf("Question saved to collection successfully")
}

func TestSaveQuestionTool_Validation(t *testing.T) {
	tests := []struct {
		name          string
		tool          *SaveQuestionTool
		expectedError string
	}{
		{
			name: "Missing name",
			tool: &SaveQuestionTool{
				Query: "SELECT 1",
				DatabaseTool: DatabaseTool{
					DatabaseID: 1,
				},
			},
			expectedError: "name is required",
		},
		{
			name: "Missing query",
			tool: &SaveQuestionTool{
				Name: "Test",
				DatabaseTool: DatabaseTool{
					DatabaseID: 1,
				},
			},
			expectedError: "query is required",
		},
		{
			name: "Missing database ID",
			tool: &SaveQuestionTool{
				Name:  "Test",
				Query: "SELECT 1",
			},
			expectedError: "database_id is required",
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

func TestSaveQuestionTool_InvalidQuery(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err)

	tool := &SaveQuestionTool{
		Name:  "Invalid Query Test",
		Query: "SELECT * FROM non_existent_table",
		DatabaseTool: DatabaseTool{
			DatabaseID: 1,
		},
	}

	ctx := context.Background()
	result, err := tool.Handle(ctx)

	// Metabase might still save the question even with invalid SQL
	// It validates when running, not when saving
	if err == nil {
		t.Log("Metabase saved the question despite invalid SQL (validates on run, not save)")
		response, ok := result.(string)
		require.True(t, ok)
		assert.Contains(t, response, "Question saved successfully!")
	} else {
		t.Logf("Metabase rejected invalid query: %v", err)
	}
}
