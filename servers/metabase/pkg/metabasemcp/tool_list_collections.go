package metabasemcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool(func() *ListCollectionsTool {
		return &ListCollectionsTool{}
	}))
}

type ListCollectionsTool struct {
	_ mcpcommon.ToolInfo `name:"list_collections" title:"List Collections" description:"List all available collections in Metabase" destructive:"false" readonly:"true"`
	MetabaseTool
}

type CollectionResponse struct {
	ID              interface{} `json:"id"` // Can be string ("root") or int
	Name            string      `json:"name"`
	Description     *string     `json:"description"`
	Color           string      `json:"color"`
	ParentID        interface{} `json:"parent_id"` // Can be string or int
	Location        string      `json:"location"`
	Namespace       *string     `json:"namespace"`
	PersonalOwnerID *int        `json:"personal_owner_id"`
	IsPersonal      bool        `json:"is_personal"`
	CanWrite        bool        `json:"can_write"`
}

func (t *ListCollectionsTool) Handle(ctx context.Context) (interface{}, error) {
	// Fetch collections
	collections, err := Get[[]CollectionResponse](ctx, "/api/collection/")
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	// Format as markdown table
	var output strings.Builder
	output.WriteString("| ID | Name | Description | Location | Type |\n")
	output.WriteString("| --- | --- | --- | --- | --- |\n")

	for _, col := range collections {
		// Format ID (can be string or int)
		idStr := fmt.Sprintf("%v", col.ID)

		// Determine collection type
		collectionType := "Regular"
		if col.IsPersonal {
			collectionType = "Personal"
		}
		if col.Namespace != nil && *col.Namespace == "snippets" {
			collectionType = "Snippets"
		}
		if idStr == "1" {
			collectionType = "Trash"
		}
		if idStr == "root" {
			collectionType = "Root"
		}
		if !col.CanWrite {
			collectionType += " (Read-only)"
		}

		// Format description
		description := ""
		if col.Description != nil && *col.Description != "" {
			description = *col.Description
		}

		// Escape pipe characters in text fields
		name := strings.ReplaceAll(col.Name, "|", "\\|")
		description = strings.ReplaceAll(description, "|", "\\|")
		location := strings.ReplaceAll(col.Location, "|", "\\|")

		output.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			idStr,
			name,
			description,
			location,
			collectionType,
		))
	}

	output.WriteString(fmt.Sprintf("\n*%d collections found*\n", len(collections)))

	return output.String(), nil
}
