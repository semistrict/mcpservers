package metabasemcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetSchemaTool(t *testing.T) {
	// Skip test if not explicitly requested
	if os.Getenv("RUN_METABASE_TESTS") != "1" {
		t.Skip("Skipping Metabase integration test. Set RUN_METABASE_TESTS=1 to run.")
	}

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

	ctx := context.Background()

	t.Run("List All Tables", func(t *testing.T) {
		tool := &GetSchemaTool{
			DatabaseID: 1, // Sample Database
			// No table name - should list all tables
		}

		result, err := tool.Handle(ctx)
		if err != nil {
			t.Fatalf("Failed to get schema: %v", err)
		}

		markdown, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string result, got %T", result)
		}

		t.Logf("Tables list:\n%s", markdown)

		// Check for expected content
		if !strings.Contains(markdown, "## Tables in") {
			t.Error("Missing tables header")
		}
		if !strings.Contains(markdown, "| Table Name | Schema | Active | Fields | Description |") {
			t.Error("Missing table headers")
		}
		// Sample database should have PRODUCTS table
		if !strings.Contains(markdown, "PRODUCTS") {
			t.Error("Missing PRODUCTS table")
		}
	})

	t.Run("Get Specific Table Fields", func(t *testing.T) {
		tool := &GetSchemaTool{
			DatabaseID: 1,
			TableNames: "PRODUCTS",
		}

		result, err := tool.Handle(ctx)
		if err != nil {
			t.Fatalf("Failed to get table fields: %v", err)
		}

		markdown, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string result, got %T", result)
		}

		t.Logf("PRODUCTS table fields:\n%s", markdown)

		// Check for expected content
		if !strings.Contains(markdown, "## Fields in") {
			t.Error("Missing fields header")
		}
		if !strings.Contains(markdown, "| Field Name | Type | Nullable | Description |") {
			t.Error("Missing field headers")
		}
		// Should have ID field
		if !strings.Contains(markdown, "ID") {
			t.Error("Missing ID field")
		}
		// Should have PRICE field
		if !strings.Contains(markdown, "PRICE") {
			t.Error("Missing PRICE field")
		}
	})

	t.Run("Get Multiple Tables Fields", func(t *testing.T) {
		tool := &GetSchemaTool{
			DatabaseID: 1,
			TableNames: "PRODUCTS, ORDERS",
		}

		result, err := tool.Handle(ctx)
		if err != nil {
			t.Fatalf("Failed to get table fields: %v", err)
		}

		markdown, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string result, got %T", result)
		}

		t.Logf("Multiple tables fields:\n%s", markdown)

		// Check for both tables
		if !strings.Contains(markdown, "PRODUCTS") {
			t.Error("Missing PRODUCTS table")
		}
		if !strings.Contains(markdown, "ORDERS") {
			t.Error("Missing ORDERS table")
		}
		// Check for separator
		if !strings.Contains(markdown, "---") {
			t.Error("Missing table separator")
		}
	})

	t.Run("Non-existent Table", func(t *testing.T) {
		tool := &GetSchemaTool{
			DatabaseID: 1,
			TableNames: "NON_EXISTENT_TABLE",
		}

		_, err := tool.Handle(ctx)
		if err == nil {
			t.Error("Expected error for non-existent table")
		} else {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("Mixed Valid and Invalid Tables", func(t *testing.T) {
		tool := &GetSchemaTool{
			DatabaseID: 1,
			TableNames: "PRODUCTS, NON_EXISTENT, ORDERS",
		}

		_, err := tool.Handle(ctx)
		if err == nil {
			t.Error("Expected error for non-existent table")
		} else {
			if !strings.Contains(err.Error(), "NON_EXISTENT") {
				t.Error("Error should mention the non-existent table")
			}
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("Invalid Database", func(t *testing.T) {
		tool := &GetSchemaTool{
			DatabaseID: 9999,
		}

		_, err := tool.Handle(ctx)
		if err == nil {
			t.Error("Expected error for invalid database")
		} else {
			t.Logf("Got expected error: %v", err)
		}
	})
}
