package mcpcommon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"reflect"
	"strings"
)

func ReflectTool[T ToolHandler]() server.ServerTool {
	var example T
	toolType := reflect.TypeOf(example)

	// If T is a pointer type, get the element type
	if toolType.Kind() == reflect.Ptr {
		toolType = toolType.Elem()
	}

	// Get tool metadata from ToolInfo field
	toolName, toolTitle, toolDescription, isDestructive := parseToolInfo(toolType)

	// Create the tool with basic info
	options := []mcp.ToolOption{
		mcp.WithDescription(toolDescription),
		mcp.WithDestructiveHintAnnotation(isDestructive),
	}

	// Add title if provided
	if toolTitle != "" {
		options = append(options, mcp.WithTitleAnnotation(toolTitle))
	}

	// Add properties from struct fields
	options = append(options, parseToolProperties(toolType)...)

	tool := mcp.NewTool(toolName, options...)

	return server.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Create new instance and populate from request arguments
			var toolInstance T

			// Handle both pointer and value types
			originalType := reflect.TypeOf(toolInstance)
			if originalType.Kind() == reflect.Ptr {
				// T is already a pointer type
				toolInstance = reflect.New(originalType.Elem()).Interface().(T)
			} else {
				// T is a value type, create a pointer to it
				ptr := reflect.New(originalType)
				toolInstance = ptr.Interface().(T)
			}

			// Unmarshal arguments into the tool struct
			if err := unmarshalArguments(toolInstance, request.GetArguments()); err != nil {
				return nil, fmt.Errorf("failed to unmarshal arguments: %v", err)
			}

			// Call the handler
			result, err := toolInstance.Handle(ctx)
			if err != nil {
				return nil, err
			}

			// Convert result to CallToolResult
			return convertResult(result), nil
		},
	}
}

func parseToolInfo(toolType reflect.Type) (name, title, description string, destructive bool) {
	for i := 0; i < toolType.NumField(); i++ {
		field := toolType.Field(i)
		if field.Type == reflect.TypeOf(ToolInfo{}) {
			name = field.Tag.Get("name")
			title = field.Tag.Get("title")
			description = field.Tag.Get("description")
			destructive = field.Tag.Get("destructive") == "true"
			return
		}
	}

	// Fallback to type name if no ToolInfo found
	name = strings.ToLower(toolType.Name())
	description = "Tool generated from " + toolType.Name()
	return
}

func parseToolProperties(toolType reflect.Type) []mcp.ToolOption {
	var options []mcp.ToolOption

	for i := 0; i < toolType.NumField(); i++ {
		field := toolType.Field(i)

		// Skip ToolInfo fields and unexported fields
		if field.Type == reflect.TypeOf(ToolInfo{}) || !field.IsExported() {
			continue
		}

		// Skip embedded structs - we'll handle their fields recursively
		if field.Anonymous {
			options = append(options, parseToolProperties(field.Type)...)
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		fieldName := strings.Split(jsonTag, ",")[0]
		description := field.Tag.Get("description")
		required := strings.Contains(field.Tag.Get("json"), "required")

		// Add property based on field type
		switch field.Type.Kind() {
		case reflect.String:
			opt := mcp.WithString(fieldName, mcp.Description(description))
			if required {
				opt = mcp.WithString(fieldName, mcp.Description(description), mcp.Required())
			}
			options = append(options, opt)
		case reflect.Bool:
			options = append(options, mcp.WithBoolean(fieldName, mcp.Description(description)))
		case reflect.Int, reflect.Int64, reflect.Float64:
			opt := mcp.WithNumber(fieldName, mcp.Description(description))
			if required {
				opt = mcp.WithNumber(fieldName, mcp.Description(description), mcp.Required())
			}
			options = append(options, opt)
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.String {
				options = append(options, mcp.WithArray(fieldName, mcp.Description(description)))
			}
		}
	}

	return options
}

func unmarshalArguments(tool interface{}, arguments map[string]interface{}) error {
	// Convert arguments to JSON and back to populate the struct
	jsonData, err := json.Marshal(arguments)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, tool)
}

func convertResult(result interface{}) *mcp.CallToolResult {
	switch v := result.(type) {
	case string:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: v,
				},
			},
		}
	case *mcp.CallToolResult:
		return v
	default:
		// Marshal to JSON and return as text
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error marshaling result: %v", err),
					},
				},
				IsError: true,
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(data),
				},
			},
		}
	}
}

// ToolHandler implements a tool.
type ToolHandler interface {
	Handle(ctx context.Context) (interface{}, error)
}

// ToolInfo is uses as the type of dummy field to annotate the tool itself with struct tags.
type ToolInfo struct{}
