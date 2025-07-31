package metabasemcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool(func() *GetSchemaTool {
		return &GetSchemaTool{}
	}))
}

type GetSchemaTool struct {
	_ mcpcommon.ToolInfo `name:"get_schema" title:"Get Schema" description:"Get database schema information. Lists all tables if no table names provided, or lists fields for specific tables" destructive:"false" readonly:"true"`
	MetabaseTool
	DatabaseID int    `json:"database_id" description:"ID of the database"`
	TableNames string `json:"table_names,omitempty" description:"Optional comma-separated list of table names to get fields for. If not provided, lists all tables"`
}

// DatabaseMetadata represents the response from GET /api/database/{id}
type DatabaseMetadata struct {
	ID     int               `json:"id"`
	Name   string            `json:"name"`
	Tables []TableWithFields `json:"tables"`
}

// TableWithFields represents a table with its fields
type TableWithFields struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Schema      string  `json:"schema"`
	Description string  `json:"description"`
	Active      bool    `json:"active"`
	Fields      []Field `json:"fields"`
}

func (t *GetSchemaTool) Handle(ctx context.Context) (interface{}, error) {
	if t.DatabaseID == 0 {
		return nil, fmt.Errorf("database_id is required")
	}

	// Get database metadata including tables and fields
	endpoint := fmt.Sprintf("/api/database/%d?include=tables.fields", t.DatabaseID)
	metadata, err := Get[DatabaseMetadata](ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get database metadata: %w", err)
	}

	// If no table names specified, return list of all tables
	if t.TableNames == "" {
		return formatTablesAsMarkdown(metadata)
	}

	// Parse comma-separated table names
	tableNames := strings.Split(t.TableNames, ",")
	for i := range tableNames {
		tableNames[i] = strings.TrimSpace(tableNames[i])
	}

	// Find the requested tables
	var targetTables []*TableWithFields
	var notFoundTables []string

	for _, tableName := range tableNames {
		found := false
		for i := range metadata.Tables {
			if strings.EqualFold(metadata.Tables[i].Name, tableName) {
				targetTables = append(targetTables, &metadata.Tables[i])
				found = true
				break
			}
		}
		if !found {
			notFoundTables = append(notFoundTables, tableName)
		}
	}

	if len(notFoundTables) > 0 {
		return nil, fmt.Errorf("table(s) not found in database: %s", strings.Join(notFoundTables, ", "))
	}

	// Return fields for the specific tables
	return formatMultipleTablesAsMarkdown(metadata.Name, targetTables)
}

func formatTablesAsMarkdown(metadata DatabaseMetadata) (string, error) {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("## Tables in %s\n\n", metadata.Name))
	builder.WriteString("| Table Name | Schema | Active | Fields | Description |\n")
	builder.WriteString("| --- | --- | --- | --- | --- |\n")

	for _, table := range metadata.Tables {
		schema := table.Schema
		if schema == "" {
			schema = "-"
		}
		description := table.Description
		if description == "" {
			description = "-"
		}

		builder.WriteString(fmt.Sprintf("| %s | %s | %v | %d | %s |\n",
			table.Name,
			schema,
			table.Active,
			len(table.Fields),
			description,
		))
	}

	builder.WriteString(fmt.Sprintf("\n*Total: %d tables*\n", len(metadata.Tables)))

	return builder.String(), nil
}

func formatFieldsAsMarkdown(dbName string, table *TableWithFields) (string, error) {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("## Fields in %s.%s\n\n", dbName, table.Name))

	if table.Description != "" {
		builder.WriteString(fmt.Sprintf("*%s*\n\n", table.Description))
	}

	builder.WriteString("| Field Name | Type | Nullable | Description |\n")
	builder.WriteString("| --- | --- | --- | --- |\n")

	for _, field := range table.Fields {
		fieldType := field.BaseType
		if fieldType == "" {
			fieldType = field.Type
		}

		nullable := "Yes"
		if field.SemanticType != nil && *field.SemanticType == "type/PK" {
			nullable = "No (PK)"
		}

		description := field.Description
		if description == "" {
			description = "-"
		}

		builder.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			field.Name,
			fieldType,
			nullable,
			description,
		))
	}

	builder.WriteString(fmt.Sprintf("\n*Total: %d fields*\n", len(table.Fields)))

	return builder.String(), nil
}

func formatMultipleTablesAsMarkdown(dbName string, tables []*TableWithFields) (string, error) {
	var builder strings.Builder

	for i, table := range tables {
		if i > 0 {
			builder.WriteString("\n---\n\n")
		}

		tableMarkdown, err := formatFieldsAsMarkdown(dbName, table)
		if err != nil {
			return "", err
		}
		builder.WriteString(tableMarkdown)
	}

	return builder.String(), nil
}
