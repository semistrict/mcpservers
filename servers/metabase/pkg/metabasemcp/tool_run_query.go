package metabasemcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool(func() *RunQueryTool {
		return &RunQueryTool{}
	}))
}

type RunQueryTool struct {
	_ mcpcommon.ToolInfo `name:"run_query" title:"Run Query" description:"Run a native SQL query against a database" destructive:"false" readonly:"true"`
	MetabaseTool
	DatabaseID int    `json:"database_id" description:"ID of the database to query"`
	Query      string `json:"query" description:"SQL query to execute"`
	OutputFile string `json:"output_file,omitempty" description:"Path to write query results (optional)"`
	Format     string `json:"format,omitempty" description:"Output format for file: jsonl, csv, or markdown (defaults to jsonl)"`
}

type QueryRequest struct {
	Database int    `json:"database"`
	Native   Native `json:"native"`
	Type     string `json:"type"`
}

type Native struct {
	Query string `json:"query"`
}

func (t *RunQueryTool) Handle(ctx context.Context) (interface{}, error) {
	if t.DatabaseID == 0 {
		return nil, fmt.Errorf("database_id is required")
	}
	if t.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	request := QueryRequest{
		Database: t.DatabaseID,
		Type:     "native",
		Native: Native{
			Query: t.Query,
		},
	}

	// Execute the query
	result, err := Post[QueryResponse](ctx, "/api/dataset", request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Check if query failed
	if result.Status == "failed" {
		// Build detailed error message
		errorMsg := fmt.Sprintf("Query failed: %s", result.Error)
		if result.ErrorType != "" {
			errorMsg += fmt.Sprintf("\nError type: %s", result.ErrorType)
		}
		if result.State != "" {
			errorMsg += fmt.Sprintf("\nSQL State: %s", result.State)
		}
		if result.Class != "" {
			errorMsg += fmt.Sprintf("\nException class: %s", result.Class)
		}
		return nil, fmt.Errorf("%s", errorMsg)
	} else if result.Status != "completed" {
		// Handle other non-completed statuses
		return nil, fmt.Errorf("query returned unexpected status: %s", result.Status)
	}

	// If output file is specified, write to file
	if t.OutputFile != "" {
		// Default to JSONL if no format specified
		format := t.Format
		if format == "" {
			format = "jsonl"
		}

		// Format the data for file output
		fileContent, err := FormatQueryResult(result, format)
		if err != nil {
			return nil, fmt.Errorf("failed to format output for file: %w", err)
		}

		// Create directory if needed
		dir := filepath.Dir(t.OutputFile)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		// Write to file
		if err := os.WriteFile(t.OutputFile, []byte(fileContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write output file: %w", err)
		}
	}

	// Always return markdown for display
	markdown, err := FormatQueryResult(result, "markdown")
	if err != nil {
		return nil, fmt.Errorf("failed to format output for display: %w", err)
	}

	// Add file info to markdown if file was written
	if t.OutputFile != "" {
		format := t.Format
		if format == "" {
			format = "jsonl"
		}
		markdown += fmt.Sprintf("\n*Results written to %s in %s format*\n", t.OutputFile, format)
	}

	return markdown, nil
}
