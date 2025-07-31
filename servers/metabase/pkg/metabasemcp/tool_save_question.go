package metabasemcp

import (
	"context"
	"fmt"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool(func() *SaveQuestionTool {
		return &SaveQuestionTool{}
	}))
}

type SaveQuestionTool struct {
	_ mcpcommon.ToolInfo `name:"save_question" title:"Save Question" description:"Save a SQL query as a question (card) in Metabase" destructive:"false" readonly:"false"`
	DatabaseTool
	Name         string `json:"name" description:"Name of the question"`
	Query        string `json:"query" description:"SQL query to save"`
	Description  string `json:"description,omitempty" description:"Optional description of the question"`
	Display      string `json:"display,omitempty" description:"Display type: table, bar, line, scalar, smartscalar, pie, area, combo, scatter, funnel, progress, number, trend, gauge, row, waterfall, map, pivot (defaults to table)"`
	CollectionID int    `json:"collection_id,omitempty" description:"Optional ID of the collection to save the question in (omit for personal collection)"`
}

type SaveQuestionRequest struct {
	Name                  string                   `json:"name"`
	DatasetQuery          SaveQuestionDatasetQuery `json:"dataset_query"`
	Display               string                   `json:"display"`
	Description           *string                  `json:"description,omitempty"`
	CollectionID          *int                     `json:"collection_id"`
	VisualizationSettings map[string]interface{}   `json:"visualization_settings"`
}

type SaveQuestionDatasetQuery struct {
	Database int                `json:"database"`
	Type     string             `json:"type"`
	Native   SaveQuestionNative `json:"native"`
}

type SaveQuestionNative struct {
	Query string `json:"query"`
}

type CardResponse struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	DatabaseID   int     `json:"database_id"`
	CollectionID *int    `json:"collection_id"`
	Display      string  `json:"display"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func (t *SaveQuestionTool) Handle(ctx context.Context) (interface{}, error) {
	if t.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if t.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if t.DatabaseID == 0 {
		return nil, fmt.Errorf("database_id is required")
	}

	// Default display type to table
	display := t.Display
	if display == "" {
		display = "table"
	}

	// Prepare description pointer
	var description *string
	if t.Description != "" {
		description = &t.Description
	}

	var collectionID *int
	if t.CollectionID == 0 {
		collectionID = nil
	} else {
		collectionID = &t.CollectionID
	}

	request := SaveQuestionRequest{
		Name: t.Name,
		DatasetQuery: SaveQuestionDatasetQuery{
			Database: t.DatabaseID,
			Type:     "native",
			Native: SaveQuestionNative{
				Query: t.Query,
			},
		},
		Display:               display,
		Description:           description,
		CollectionID:          collectionID,
		VisualizationSettings: make(map[string]interface{}),
	}

	// Save the question
	result, err := Post[CardResponse](ctx, "/api/card", request)
	if err != nil {
		return nil, fmt.Errorf("failed to save question: %w", err)
	}

	// Format response
	response := "Question saved successfully!\n\n"
	response += fmt.Sprintf("ID: %d\n", result.ID)
	response += fmt.Sprintf("Name: %s\n", result.Name)
	if result.Description != nil && *result.Description != "" {
		response += fmt.Sprintf("Description: %s\n", *result.Description)
	}
	response += fmt.Sprintf("Database ID: %d\n", result.DatabaseID)
	if result.CollectionID != nil {
		response += fmt.Sprintf("Collection ID: %d\n", *result.CollectionID)
	} else {
		response += "Collection: Personal Collection\n"
	}
	response += fmt.Sprintf("Display Type: %s\n", result.Display)
	response += fmt.Sprintf("Created: %s\n", result.CreatedAt)

	return response, nil
}
