package metabasemcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool(func() *RawAPITool {
		return &RawAPITool{}
	}))
}

type RawAPITool struct {
	_ mcpcommon.ToolInfo `name:"raw_api" title:"Raw API Call" description:"Make a raw API call to Metabase (for debugging/testing)" destructive:"false" readonly:"false"`
	MetabaseTool
	Method      string `json:"method" description:"HTTP method (GET, POST, PUT, DELETE)"`
	Path        string `json:"path" description:"API path (e.g., /api/database/)"`
	Body        string `json:"body,omitempty" description:"Request body as JSON string (for POST/PUT requests)"`
	ContentType string `json:"content_type,omitempty" description:"Content-Type header (defaults to application/json for POST/PUT)"`
}

func (t *RawAPITool) Handle(ctx context.Context) (interface{}, error) {
	if t.Method == "" {
		return nil, fmt.Errorf("method is required")
	}
	if t.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	client := getClient()
	if client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	var bodyBytes []byte
	if t.Body != "" {
		bodyBytes = []byte(t.Body)
	}

	// Default content type for POST/PUT
	contentType := t.ContentType
	if contentType == "" && (t.Method == "POST" || t.Method == "PUT") && bodyBytes != nil {
		contentType = "application/json"
	}

	respBody, err := client.Request(ctx, t.Method, t.Path, bodyBytes, contentType)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON, but if it fails, return as string
	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Not valid JSON, return as string
		return string(respBody), nil
	}

	return result, nil
}
