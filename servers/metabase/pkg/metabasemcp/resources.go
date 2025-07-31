package metabasemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"log/slog"
)

func loadDatabaseResources(ctx context.Context, addResource func(resource mcp.Resource, handlerFunc server.ResourceHandlerFunc)) {
	slog.Info("loading database resources asynchronously")

	// Fetch databases
	response, err := Get[databaseListResponse](ctx, "/api/database/")
	if err != nil {
		slog.Error("failed to load databases for resources", "error", err)
		return
	}

	// Add each database as a resource
	for _, db := range response.Data {
		// Create resource URI
		uri := fmt.Sprintf("metabase://database/%d", db.ID)

		// Create resource
		resource := mcp.NewResource(
			uri,
			db.Name,
			mcp.WithResourceDescription(fmt.Sprintf("Metabase database: %s (%s)", db.Description, db.Engine)),
			mcp.WithMIMEType("application/json"),
		)

		// Add resource with handler
		addResource(resource, createDatabaseResourceHandler(db.ID))

		slog.Info("added database resource", "name", db.Name, "id", db.ID, "uri", uri)
	}

	slog.Info("finished loading database resources", "count", len(response.Data))
}

func createDatabaseResourceHandler(databaseID int) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Get fresh database metadata
		endpoint := fmt.Sprintf("/api/database/%d?include=tables", databaseID)
		metadata, err := Get[DatabaseMetadata](ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to get database metadata: %w", err)
		}

		// Create summary
		summary := struct {
			ID         int      `json:"id"`
			Name       string   `json:"name"`
			Engine     string   `json:"engine"`
			TableCount int      `json:"table_count"`
			TableNames []string `json:"table_names"`
		}{
			ID:         databaseID,
			Name:       metadata.Name,
			Engine:     "", // Would need to fetch full database info
			TableCount: len(metadata.Tables),
			TableNames: make([]string, 0, len(metadata.Tables)),
		}

		for _, table := range metadata.Tables {
			summary.TableNames = append(summary.TableNames, table.Name)
		}

		// Convert to JSON
		jsonData, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal database info: %w", err)
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     string(jsonData),
			},
		}, nil
	}
}
