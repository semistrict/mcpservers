package metabasemcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRawAPITool_ListDatabases(t *testing.T) {
	skipIfNoMetabase(t)

	// Use test server on localhost:3141
	serverURL := "http://localhost:3141"

	// Use the default cookies file location
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	cookiesFile := filepath.Join(home, ".metabase", "cookies.txt")

	// Initialize the client
	err = InitializeClient(serverURL, cookiesFile)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Create and execute the raw API tool
	tool := &RawAPITool{
		Method: "GET",
		Path:   "/api/database/",
	}
	ctx := context.Background()

	result, err := tool.Handle(ctx)
	if err != nil {
		t.Fatalf("Failed to make raw API call: %v", err)
	}

	// Pretty print the result to see the structure
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	t.Logf("Raw API Response for GET /api/database/:\n%s", string(jsonBytes))

	// Try to understand the structure
	if resultMap, ok := result.(map[string]interface{}); ok {
		t.Logf("\nResponse is a map with keys:")
		for key := range resultMap {
			t.Logf("  - %s", key)
		}

		// Check if there's a data field
		if data, hasData := resultMap["data"]; hasData {
			t.Logf("\nFound 'data' field, type: %T", data)
			if dataArray, ok := data.([]interface{}); ok {
				t.Logf("Data is an array with %d items", len(dataArray))
			}
		}
	} else if resultArray, ok := result.([]interface{}); ok {
		t.Logf("\nResponse is an array with %d items", len(resultArray))
	} else {
		t.Logf("\nResponse type: %T", result)
	}
}
