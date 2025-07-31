package metabasemcp

import (
	"context"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool(func() *ListDatabasesTool {
		return &ListDatabasesTool{}
	}))
}

type ListDatabasesTool struct {
	_ mcpcommon.ToolInfo `name:"list_databases" title:"List Databases" description:"List all databases configured in Metabase" destructive:"false" readonly:"true"`
	MetabaseTool
}

type databaseListResponse struct {
	Data  []*Database `json:"data"`
	Total int         `json:"total"`
}

func (t *ListDatabasesTool) Handle(ctx context.Context) (interface{}, error) {
	response, err := Get[databaseListResponse](ctx, "/api/database/")
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}
