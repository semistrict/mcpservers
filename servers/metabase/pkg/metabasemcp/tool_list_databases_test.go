package metabasemcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListDatabasesTool(t *testing.T) {
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

	// Create and execute the tool
	tool := &ListDatabasesTool{}
	ctx := context.Background()

	result, err := tool.Handle(ctx)
	if err != nil {
		t.Fatalf("Failed to list databases: %v", err)
	}

	// Check that we got a result
	databases, ok := result.([]*Database)
	if !ok {
		t.Fatalf("Expected result to be []*Database, got %T", result)
	}

	// Log the databases found
	t.Logf("Found %d databases", len(databases))
	for _, db := range databases {
		t.Logf("  - Database: ID=%d, Name=%s, Engine=%s", db.ID, db.Name, db.Engine)
	}

	// At minimum, we should have the sample database
	if len(databases) == 0 {
		t.Error("Expected at least one database (sample database)")
	}
}

// TestListDatabasesToolWithInvalidServer tests error handling with invalid server
func TestListDatabasesToolWithInvalidServer(t *testing.T) {
	// This test doesn't require a running server
	serverURL := "http://invalid-server:9999"
	cookiesFile := "/tmp/fake-cookies.txt"

	// Create a fake cookies file
	err := os.WriteFile(cookiesFile, []byte("test\tcookie\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test cookies file: %v", err)
	}
	defer os.Remove(cookiesFile)

	// Initialize the client
	err = InitializeClient(serverURL, cookiesFile)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Create and execute the tool
	tool := &ListDatabasesTool{}
	ctx := context.Background()

	_, err = tool.Handle(ctx)
	if err == nil {
		t.Error("Expected error when connecting to invalid server, got nil")
	}
}
