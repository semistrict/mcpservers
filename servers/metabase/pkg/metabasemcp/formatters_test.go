package metabasemcp

import (
	"strings"
	"testing"
)

func TestFormatters(t *testing.T) {
	// Create a sample query response
	response := QueryResponse{
		Status:      "completed",
		RowCount:    2,
		RunningTime: 25,
		Data: QueryData{
			Cols: []ColumnMetadata{
				{Name: "ID", DisplayName: "ID", BaseType: "type/BigInteger"},
				{Name: "NAME", DisplayName: "Name", BaseType: "type/Text"},
				{Name: "PRICE", DisplayName: "Price", BaseType: "type/Float"},
			},
			Rows: [][]interface{}{
				{1, "Product One", 19.99},
				{2, "Product Two", 29.99},
			},
		},
	}

	t.Run("JSONL Format", func(t *testing.T) {
		result, err := formatJSONL(response)
		if err != nil {
			t.Fatalf("Failed to format as JSONL: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(result), "\n")
		if len(lines) != 2 {
			t.Errorf("Expected 2 lines, got %d", len(lines))
		}

		// Check first line contains expected fields
		if !strings.Contains(lines[0], `"ID":1`) {
			t.Errorf("First line missing ID field")
		}
		if !strings.Contains(lines[0], `"NAME":"Product One"`) {
			t.Errorf("First line missing NAME field")
		}
		if !strings.Contains(lines[0], `"PRICE":19.99`) {
			t.Errorf("First line missing PRICE field")
		}

		t.Logf("JSONL output:\n%s", result)
	})

	t.Run("CSV Format", func(t *testing.T) {
		result, err := formatCSV(response)
		if err != nil {
			t.Fatalf("Failed to format as CSV: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(result), "\n")
		if len(lines) != 3 { // header + 2 data rows
			t.Errorf("Expected 3 lines, got %d", len(lines))
		}

		// Check header
		if lines[0] != "ID,NAME,PRICE" {
			t.Errorf("Unexpected header: %s", lines[0])
		}

		// Check first data row
		if lines[1] != "1,Product One,19.99" {
			t.Errorf("Unexpected first row: %s", lines[1])
		}

		t.Logf("CSV output:\n%s", result)
	})

	t.Run("Markdown Format", func(t *testing.T) {
		result, err := formatMarkdown(response)
		if err != nil {
			t.Fatalf("Failed to format as Markdown: %v", err)
		}

		// Check for table structure
		if !strings.Contains(result, "| ID | NAME | PRICE |") {
			t.Error("Missing table header")
		}
		if !strings.Contains(result, "| --- | --- | --- |") {
			t.Error("Missing table separator")
		}
		if !strings.Contains(result, "| 1 | Product One | 19.99 |") {
			t.Error("Missing first data row")
		}
		if !strings.Contains(result, "*2 rows returned in 25 ms*") {
			t.Error("Missing summary")
		}

		t.Logf("Markdown output:\n%s", result)
	})

	t.Run("Markdown with Pipe Characters", func(t *testing.T) {
		// Test escaping of pipe characters
		response.Data.Rows = [][]interface{}{
			{1, "Product | With Pipe", 19.99},
		}

		result, err := formatMarkdown(response)
		if err != nil {
			t.Fatalf("Failed to format as Markdown: %v", err)
		}

		// Check that pipe is escaped
		if !strings.Contains(result, "Product \\| With Pipe") {
			t.Error("Pipe character not properly escaped")
		}

		t.Logf("Markdown with escaped pipes:\n%s", result)
	})
}

func TestFormatQueryResult(t *testing.T) {
	response := QueryResponse{
		Status:      "completed",
		RowCount:    1,
		RunningTime: 10,
		Data: QueryData{
			Cols: []ColumnMetadata{
				{Name: "TEST", DisplayName: "Test", BaseType: "type/Text"},
			},
			Rows: [][]interface{}{
				{"value"},
			},
		},
	}

	tests := []struct {
		format string
		valid  bool
	}{
		{"jsonl", true},
		{"JSONL", true},
		{"csv", true},
		{"CSV", true},
		{"markdown", true},
		{"md", true},
		{"MD", true},
		{"xml", false},
		{"", false},
	}

	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			result, err := FormatQueryResult(response, test.format)
			if test.valid {
				if err != nil {
					t.Errorf("Expected success for format %s, got error: %v", test.format, err)
				}
				if result == "" {
					t.Errorf("Expected non-empty result for format %s", test.format)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for format %s, got success", test.format)
				}
			}
		})
	}
}
