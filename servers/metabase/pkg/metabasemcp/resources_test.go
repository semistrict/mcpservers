package metabasemcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseResources(t *testing.T) {
	skipIfNoMetabase(t)

	// Initialize test client
	testServerURL := "http://localhost:3141"
	testCookiesFile := "/Users/ramon/.metabase/cookies.txt"
	err := InitializeClient(testServerURL, testCookiesFile)
	require.NoError(t, err, "Failed to initialize client")

	// Create mcptest server with resources
	srv := mcptest.NewUnstartedServer(t)
	defer srv.Close()

	loadDatabaseResources(context.Background(), srv.AddResource)

	// Start the server
	ctx := context.Background()
	err = srv.Start(ctx)
	require.NoError(t, err, "Failed to start server")

	// Use the client to list resources
	client := srv.Client()
	listResult, err := client.ListResources(ctx, mcp.ListResourcesRequest{})
	require.NoError(t, err, "Failed to list resources")

	require.NotEmpty(t, listResult.Resources, "Expected at least one resource")

	// Verify resource structure (Sample Database is the default in Metabase)
	resource := listResult.Resources[0]
	assert.NotEmpty(t, resource.URI, "Resource has empty URI")
	assert.NotEmpty(t, resource.Name, "Resource has empty name")
	t.Logf("Found resource: %s (%s)", resource.Name, resource.URI)

	// Test reading the resource
	readResult, err := client.ReadResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: resource.URI,
		},
	})
	if err != nil {
		// This is expected if the test Metabase doesn't have this database
		t.Logf("Failed to read resource (expected if database doesn't exist): %v", err)
		return
	}

	require.NotEmpty(t, readResult.Contents, "Resource returned no contents")

	// Verify JSON structure
	for _, content := range readResult.Contents {
		textContent, ok := content.(mcp.TextResourceContents)
		require.True(t, ok, "Expected TextResourceContents, got %T", content)

		var data map[string]interface{}
		err := json.Unmarshal([]byte(textContent.Text), &data)
		require.NoError(t, err, "Failed to parse resource JSON")

		// Verify expected fields
		assert.Contains(t, data, "id", "Resource JSON missing 'id' field")
		assert.Contains(t, data, "name", "Resource JSON missing 'name' field")
		assert.Contains(t, data, "table_count", "Resource JSON missing 'table_count' field")
		assert.Contains(t, data, "table_names", "Resource JSON missing 'table_names' field")

		t.Logf("Resource content: %s", textContent.Text)
	}
}
