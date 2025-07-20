package mcpcommon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"log"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

func ReflectTool[T ToolHandler](constructor func() T) server.ServerTool {
	example := constructor()
	toolType := reflect.TypeOf(example)

	// If T is a pointer type, get the element type
	if toolType.Kind() == reflect.Ptr {
		toolType = toolType.Elem()
	}

	// Get tool metadata from ToolInfo field
	toolName, toolTitle, toolDescription, isDestructive, isReadOnly := parseToolInfo(toolType)
	if len(toolName) == 0 {
		toolName = toolType.Name()
	}

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
			var toolInstance = constructor()
			return InvokeReflectTool(ctx, toolName, toolInstance, request)
		},
	}
}

func InvokeReflectTool(ctx context.Context, toolName string, toolInstance ToolHandler, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tool panic: %s", r)
		}
	}()

	if err := unmarshalArguments(toolInstance, request.GetArguments()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %v", err)
	}

	ctx = withCallToolRequest(ctx, &request)

	var rawResult any
	slog.DebugContext(ctx, "calling tool", "tool", toolName)
	rawResult, err = toolInstance.Handle(ctx)
	if err != nil {

		slog.WarnContext(ctx, "tool returned error", "err", err)
		return convertResult(toolName, err), nil
	}

	// Convert result to CallToolResult
	return convertResult(toolName, rawResult), nil
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

var registeredStructSchemas sync.Map

func RegisterStructSchema(structName string, schemaJSON string) {
	schema := map[string]any{}
	err := json.Unmarshal([]byte(schemaJSON), &schema)
	if err != nil {
		log.Panicf("invalid json: %v", err)
	}
	registeredStructSchemas.Store(structName, schema)
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

		var paramOptions []mcp.PropertyOption
		paramOptions = append(paramOptions, mcp.Description(description))
		if required {
			paramOptions = append(paramOptions, mcp.Required())
		}

		// Validate that description doesn't contain "default:" - should use separate tag
		if strings.Contains(strings.ToLower(description), "default:") {
			panic(fmt.Sprintf("Field %s.%s: description contains 'default:' - use separate 'default' struct tag instead",
				toolType.Name(), field.Name))
		}

		if field.Type == reflect.TypeOf(json.RawMessage{}) {
			paramOptions = append(paramOptions, mcp.AdditionalProperties(true))
			paramOptions = append(paramOptions, func(m map[string]any) {
				delete(m, "properties")
			})
			options = append(options, mcp.WithObject(fieldName, paramOptions...))
			continue
		}

		// Add property based on field type
		switch field.Type.Kind() {
		case reflect.Pointer:
			element := field.Type.Elem()
			// TODO: actually implement this with reflection, for now we just allow hard-coded schemas
			val, ok := registeredStructSchemas.Load(element.Name())
			if !ok {
				log.Panicf("struct schema not registered: %s", field.Type.Name())
			}
			schema := val.(map[string]any)
			paramOptions = append(paramOptions, func(s map[string]any) {
				for k, v := range schema {
					s[k] = v
				}
			})
			options = append(options, mcp.WithObject(fieldName, paramOptions...))
			continue
		case reflect.String:
			if defaultValue != "" {
				paramOptions = append(paramOptions, mcp.DefaultString(defaultValue))
			}
			options = append(options, mcp.WithString(fieldName, paramOptions...))
			continue

		case reflect.Bool:
			if defaultValue != "" {
				if defaultValue == "true" {
					paramOptions = append(paramOptions, mcp.DefaultBool(true))
				} else if defaultValue == "false" {
					paramOptions = append(paramOptions, mcp.DefaultBool(false))
				}
			}
			options = append(options, mcp.WithBoolean(fieldName, paramOptions...))
			continue

		case reflect.Int, reflect.Int64, reflect.Float64:
			if defaultValue != "" {
				if defaultNum, err := strconv.ParseFloat(defaultValue, 64); err == nil {
					paramOptions = append(paramOptions, mcp.DefaultNumber(defaultNum))
				}
			}
			options = append(options, mcp.WithNumber(fieldName, paramOptions...))
			continue
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.String {
				paramOptions = append(paramOptions, mcp.WithStringItems())
				// Array of strings - specify items as string type
				options = append(options, mcp.WithArray(fieldName, paramOptions...))
				continue
			}
		}

		log.Panicf("don't know how to represent parameter %v", field)
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

func convertResult(toolName string, result interface{}) *mcp.CallToolResult {
	switch v := result.(type) {
	case error:
		return mcp.NewToolResultErrorFromErr(toolName, v)
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
