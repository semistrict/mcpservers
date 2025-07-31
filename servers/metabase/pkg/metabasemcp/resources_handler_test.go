package metabasemcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDatabaseResourceHandler(t *testing.T) {
	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err, "Failed to initialize client")

	// Create handler for database ID 1 (Sample Database)
	handler := createDatabaseResourceHandler(1)

	// Call the handler
	ctx := context.Background()
	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "metabase://database/1",
		},
	}

	contents, err := handler(ctx, req)
	require.NoError(t, err, "Handler returned error")

	// Verify response
	require.Len(t, contents, 1, "Expected exactly 1 content")

	textContent, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok, "Expected TextResourceContents, got %T", contents[0])

	assert.Equal(t, req.Params.URI, textContent.URI, "URI mismatch")
	assert.Equal(t, "application/json", textContent.MIMEType, "MIME type mismatch")

	// Verify JSON is valid and contains expected fields
	var data map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &data)
	require.NoError(t, err, "Failed to parse JSON")

	// Check all required fields
	assert.Equal(t, float64(1), data["id"], "ID mismatch")
	assert.Equal(t, "Sample Database", data["name"], "Name mismatch")
	assert.Contains(t, data, "engine", "Missing engine field")
	assert.Contains(t, data, "table_count", "Missing table_count field")
	assert.Contains(t, data, "table_names", "Missing table_names field")

	// Verify table_names is an array
	tableNames, ok := data["table_names"].([]interface{})
	require.True(t, ok, "table_names should be an array")
	assert.NotEmpty(t, tableNames, "table_names should not be empty")

	// Verify table count matches
	tableCount, ok := data["table_count"].(float64)
	require.True(t, ok, "table_count should be a number")
	assert.Equal(t, len(tableNames), int(tableCount), "table_count should match length of table_names")

	t.Logf("Handler returned valid JSON with %d tables", int(tableCount))
}

func TestCreateDatabaseResourceHandler_InvalidDatabase(t *testing.T) {
	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err, "Failed to initialize client")

	// Create handler for non-existent database
	handler := createDatabaseResourceHandler(99999)

	// Call the handler
	ctx := context.Background()
	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "metabase://database/99999",
		},
	}

	contents, err := handler(ctx, req)
	assert.Error(t, err, "Expected error for non-existent database")
	assert.Nil(t, contents, "Expected nil contents on error")
}
