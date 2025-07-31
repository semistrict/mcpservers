package metabasemcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunQueryTool(t *testing.T) {
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

	// Create and execute the tool with a simple query
	tool := &RunQueryTool{
		DatabaseID: 1, // Sample Database
		Query:      "SELECT * FROM PRODUCTS LIMIT 5",
	}
	ctx := context.Background()

	result, err := tool.Handle(ctx)
	if err != nil {
		t.Fatalf("Failed to run query: %v", err)
	}

	// Result should always be markdown now
	markdownResult, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string (markdown) result, got %T", result)
	}

	t.Logf("Markdown result:\n%s", markdownResult)

	// Check markdown contains expected elements
	if !strings.Contains(markdownResult, "| ID |") {
		t.Error("Markdown missing table header")
	}
	if !strings.Contains(markdownResult, "5 rows returned") {
		t.Error("Markdown missing expected row count")
	}
}

func TestRunQueryTool_WithFileOutput(t *testing.T) {
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
	tmpDir := t.TempDir()

	t.Run("Default JSONL File Output", func(t *testing.T) {
		outputFile := filepath.Join(tmpDir, "test_output.jsonl")
		tool := &RunQueryTool{
			DatabaseID: 1,
			Query:      "SELECT ID, TITLE, PRICE FROM PRODUCTS LIMIT 3",
			OutputFile: outputFile,
			// No format specified, should default to JSONL
		}

		result, err := tool.Handle(ctx)
		if err != nil {
			t.Fatalf("Failed to run query: %v", err)
		}

		// Check result is markdown
		markdownResult, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string (markdown) result, got %T", result)
		}

		// Check markdown mentions file was written
		if !strings.Contains(markdownResult, "Results written to") {
			t.Error("Markdown missing file write confirmation")
		}
		if !strings.Contains(markdownResult, "jsonl format") {
			t.Error("Markdown missing format info")
		}

		// Check file was created
		if _, err := os.Stat(outputFile); os.IsNotExist(err) {
			t.Error("Output file was not created")
		}

		// Read and check file content
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 3 {
			t.Errorf("Expected 3 JSONL lines, got %d", len(lines))
		}

		t.Logf("JSONL file content:\n%s", string(content))
	})

	t.Run("CSV File Output", func(t *testing.T) {
		outputFile := filepath.Join(tmpDir, "test_output.csv")
		tool := &RunQueryTool{
			DatabaseID: 1,
			Query:      "SELECT ID, TITLE FROM PRODUCTS LIMIT 2",
			OutputFile: outputFile,
			Format:     "csv",
		}

		result, err := tool.Handle(ctx)
		if err != nil {
			t.Fatalf("Failed to run query: %v", err)
		}

		// Check result is markdown
		markdownResult, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string (markdown) result, got %T", result)
		}

		// Check markdown mentions the file output
		if !strings.Contains(markdownResult, "csv format") {
			t.Error("Markdown missing format info")
		}

		// Check file was created
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		if !strings.Contains(string(content), "ID,TITLE") {
			t.Error("CSV file missing header")
		}

		t.Logf("CSV file content:\n%s", string(content))
	})

	t.Run("JSONL File Output", func(t *testing.T) {
		outputFile := filepath.Join(tmpDir, "test_output.jsonl")
		tool := &RunQueryTool{
			DatabaseID: 1,
			Query:      "SELECT ID, TITLE FROM PRODUCTS LIMIT 2",
			OutputFile: outputFile,
			Format:     "jsonl",
		}

		result, err := tool.Handle(ctx)
		if err != nil {
			t.Fatalf("Failed to run query: %v", err)
		}

		// Check result is markdown
		markdownResult, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string (markdown) result, got %T", result)
		}

		// Check markdown mentions the file output
		if !strings.Contains(markdownResult, "jsonl format") {
			t.Error("Markdown missing format info")
		}

		// Check file was created
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 2 {
			t.Errorf("Expected 2 JSONL lines, got %d", len(lines))
		}

		t.Logf("JSONL file content:\n%s", string(content))
	})

	t.Run("Markdown File Output", func(t *testing.T) {
		outputFile := filepath.Join(tmpDir, "subdir", "test_output.md")
		tool := &RunQueryTool{
			DatabaseID: 1,
			Query:      "SELECT ID, TITLE, CATEGORY FROM PRODUCTS WHERE ID <= 3",
			OutputFile: outputFile,
			Format:     "markdown",
		}

		result, err := tool.Handle(ctx)
		if err != nil {
			t.Fatalf("Failed to run query: %v", err)
		}

		// Check result is markdown
		markdownResult, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string (markdown) result, got %T", result)
		}

		// Check markdown mentions the file output
		if !strings.Contains(markdownResult, "markdown format") {
			t.Error("Markdown missing format info")
		}

		// Check file was created (including subdirectory)
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		if !strings.Contains(string(content), "| ID | TITLE | CATEGORY |") {
			t.Error("Markdown file missing table header")
		}

		t.Logf("Markdown file content:\n%s", string(content))
	})
}

func TestRunQueryTool_InvalidDatabase(t *testing.T) {
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

	// Try with invalid database ID
	tool := &RunQueryTool{
		DatabaseID: 9999, // Non-existent database
		Query:      "SELECT 1",
	}
	ctx := context.Background()

	_, err = tool.Handle(ctx)
	if err == nil {
		t.Error("Expected error for invalid database ID, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}
