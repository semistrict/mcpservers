package metabasemcp

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

// FormatQueryResult formats query results based on the specified format
func FormatQueryResult(response QueryResponse, format string) (string, error) {
	switch strings.ToLower(format) {
	case "jsonl":
		return formatJSONL(response)
	case "csv":
		return formatCSV(response)
	case "markdown", "md":
		return formatMarkdown(response)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// formatJSONL formats query results as JSON Lines (one JSON object per row)
func formatJSONL(response QueryResponse) (string, error) {
	var builder strings.Builder

	// Create a map for each row using column names as keys
	for _, row := range response.Data.Rows {
		rowMap := make(map[string]interface{})
		for i, col := range response.Data.Cols {
			if i < len(row) {
				rowMap[col.Name] = row[i]
			}
		}

		jsonBytes, err := json.Marshal(rowMap)
		if err != nil {
			return "", fmt.Errorf("failed to marshal row: %w", err)
		}
		builder.Write(jsonBytes)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

// formatCSV formats query results as CSV
func formatCSV(response QueryResponse) (string, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)

	// Write header row
	headers := make([]string, len(response.Data.Cols))
	for i, col := range response.Data.Cols {
		headers[i] = col.Name
	}
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write data rows
	for _, row := range response.Data.Rows {
		strRow := make([]string, len(row))
		for i, val := range row {
			strRow[i] = fmt.Sprintf("%v", val)
		}
		if err := writer.Write(strRow); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("CSV writer error: %w", err)
	}

	return builder.String(), nil
}

// formatMarkdown formats query results as a Markdown table
func formatMarkdown(response QueryResponse) (string, error) {
	var builder strings.Builder

	// Write header
	builder.WriteString("|")
	for _, col := range response.Data.Cols {
		builder.WriteString(" ")
		builder.WriteString(col.Name)
		builder.WriteString(" |")
	}
	builder.WriteString("\n")

	// Write separator
	builder.WriteString("|")
	for range response.Data.Cols {
		builder.WriteString(" --- |")
	}
	builder.WriteString("\n")

	// Write data rows
	for _, row := range response.Data.Rows {
		builder.WriteString("|")
		for _, val := range row {
			builder.WriteString(" ")
			// Escape pipe characters in values
			valStr := fmt.Sprintf("%v", val)
			valStr = strings.ReplaceAll(valStr, "|", "\\|")
			builder.WriteString(valStr)
			builder.WriteString(" |")
		}
		builder.WriteString("\n")
	}

	// Add summary
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("*%d rows returned in %d ms*\n", response.RowCount, response.RunningTime))

	return builder.String(), nil
}
