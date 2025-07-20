package mcpcommon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"reflect"
	"strconv"
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
	toolName, toolTitle, toolDescription, isDestructive, isReadOnly := parseToolInfo(toolType)

	// Create the tool with basic info
	options := []mcp.ToolOption{
		mcp.WithDescription(toolDescription),
		mcp.WithDestructiveHintAnnotation(isDestructive),
		mcp.WithReadOnlyHintAnnotation(isReadOnly),
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
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
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

			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("tool panic: %s", r)
				}
			}()

			var rawResult any
			// Call the handler
			rawResult, err = toolInstance.Handle(ctx)
			if err != nil {
				return nil, err
			}

			// Convert result to CallToolResult
			return convertResult(rawResult), nil
		},
	}
}

func parseToolInfo(toolType reflect.Type) (name, title, description string, destructive, readonly bool) {
	for i := 0; i < toolType.NumField(); i++ {
		field := toolType.Field(i)
		if field.Type == reflect.TypeOf(ToolInfo{}) {
			name = field.Tag.Get("name")
			title = field.Tag.Get("title")
			description = field.Tag.Get("description")
			destructive = field.Tag.Get("destructive") == "true"
			readonly = field.Tag.Get("readonly") == "true"
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
		required := field.Tag.Get("mcp") == "required"
		defaultValue := field.Tag.Get("default")

		// Validate that description doesn't contain "default:" - should use separate tag
		if strings.Contains(strings.ToLower(description), "default:") {
			panic(fmt.Sprintf("Field %s.%s: description contains 'default:' - use separate 'default' struct tag instead",
				toolType.Name(), field.Name))
		}

		// Add property based on field type
		switch field.Type.Kind() {
		case reflect.String:
			var paramOptions []mcp.PropertyOption
			paramOptions = append(paramOptions, mcp.Description(description))
			if required {
				paramOptions = append(paramOptions, mcp.Required())
			}
			if defaultValue != "" {
				paramOptions = append(paramOptions, mcp.DefaultString(defaultValue))
			}
			options = append(options, mcp.WithString(fieldName, paramOptions...))

		case reflect.Bool:
			var paramOptions []mcp.PropertyOption
			paramOptions = append(paramOptions, mcp.Description(description))
			if defaultValue != "" {
				if defaultValue == "true" {
					paramOptions = append(paramOptions, mcp.DefaultBool(true))
				} else if defaultValue == "false" {
					paramOptions = append(paramOptions, mcp.DefaultBool(false))
				}
			}
			options = append(options, mcp.WithBoolean(fieldName, paramOptions...))

		case reflect.Int, reflect.Int64, reflect.Float64:
			var paramOptions []mcp.PropertyOption
			paramOptions = append(paramOptions, mcp.Description(description))
			if required {
				paramOptions = append(paramOptions, mcp.Required())
			}
			if defaultValue != "" {
				if defaultNum, err := strconv.ParseFloat(defaultValue, 64); err == nil {
					paramOptions = append(paramOptions, mcp.DefaultNumber(defaultNum))
				}
			}
			options = append(options, mcp.WithNumber(fieldName, paramOptions...))
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.String {
				// Array of strings - specify items as string type
				options = append(options, mcp.WithArray(fieldName,
					mcp.Description(description),
					mcp.WithStringItems(),
				))
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
