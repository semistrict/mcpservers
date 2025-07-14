package mcpcommon

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Test tool with various parameter types and struct tags
type TestToolWithTags struct {
	ToolInfo `name:"test_tool" description:"A test tool for struct tag validation"`
	
	RequiredString string  `json:"required_string,required" description:"A required string parameter"`
	OptionalString string  `json:"optional_string" description:"An optional string parameter" default:"default_value"`
	RequiredNumber int     `json:"required_number,required" description:"A required number parameter"`
	OptionalNumber float64 `json:"optional_number" description:"An optional number parameter" default:"42.5"`
	OptionalBool   bool    `json:"optional_bool" description:"An optional boolean parameter" default:"true"`
	NoDefault      string  `json:"no_default" description:"A parameter with no default"`
}

func (t *TestToolWithTags) Handle(ctx context.Context) (interface{}, error) {
	return "test result", nil
}

func TestReflectToolWithStructTags(t *testing.T) {
	// Create the server tool using reflection
	serverTool := ReflectTool[*TestToolWithTags]()
	
	// Verify the tool was created
	if serverTool.Tool.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", serverTool.Tool.Name)
	}
	
	if serverTool.Tool.Description != "A test tool for struct tag validation" {
		t.Errorf("Expected description 'A test tool for struct tag validation', got '%s'", serverTool.Tool.Description)
	}
	
	// Verify the schema has the expected properties
	schema := serverTool.Tool.InputSchema
	
	// Check that properties exist
	if schema.Properties == nil {
		t.Fatal("Expected properties to be defined")
	}
	
	// Check required string parameter exists
	if _, exists := schema.Properties["required_string"]; !exists {
		t.Error("Expected required_string property to exist")
	}
	
	// Check optional string exists
	if _, exists := schema.Properties["optional_string"]; !exists {
		t.Error("Expected optional_string property to exist")
	}
	
	// Check required number parameter exists
	if _, exists := schema.Properties["required_number"]; !exists {
		t.Error("Expected required_number property to exist")
	}
	
	// Check optional number exists
	if _, exists := schema.Properties["optional_number"]; !exists {
		t.Error("Expected optional_number property to exist")
	}
	
	// Check optional boolean exists
	if _, exists := schema.Properties["optional_bool"]; !exists {
		t.Error("Expected optional_bool property to exist")
	}
	
	// Check parameter with no default exists
	if _, exists := schema.Properties["no_default"]; !exists {
		t.Error("Expected no_default property to exist")
	}
	
	// Check required fields are in the required array
	hasRequiredString := false
	hasRequiredNumber := false
	for _, req := range schema.Required {
		if req == "required_string" {
			hasRequiredString = true
		}
		if req == "required_number" {
			hasRequiredNumber = true
		}
	}
	
	if !hasRequiredString {
		t.Error("Expected 'required_string' to be in required fields")
	}
	
	if !hasRequiredNumber {
		t.Error("Expected 'required_number' to be in required fields")
	}
	
	// Verify we have the right number of properties (6 parameters)
	expectedProps := 6
	if len(schema.Properties) != expectedProps {
		t.Errorf("Expected %d properties, got %d", expectedProps, len(schema.Properties))
	}
	
	// Verify we have the right number of required fields (2)
	expectedRequired := 2
	if len(schema.Required) != expectedRequired {
		t.Errorf("Expected %d required fields, got %d", expectedRequired, len(schema.Required))
	}
}

func TestReflectToolHandlerExecution(t *testing.T) {
	// Create the server tool using reflection
	serverTool := ReflectTool[*TestToolWithTags]()
	
	// Create a test request with required parameters
	arguments := map[string]interface{}{
		"required_string": "test_value",
		"required_number": 123,
	}
	
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "test_tool",
			Arguments: arguments,
		},
	}
	
	// Execute the handler
	ctx := context.Background()
	result, err := serverTool.Handler(ctx, request)
	
	if err != nil {
		t.Fatalf("Handler execution failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	
	// The result should be a *mcp.CallToolResult
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}
	
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", result.Content[0])
	}
	
	if textContent.Text != "test result" {
		t.Errorf("Expected 'test result', got '%s'", textContent.Text)
	}
}

func TestReflectToolWithMissingRequiredParameter(t *testing.T) {
	// Create the server tool using reflection
	serverTool := ReflectTool[*TestToolWithTags]()
	
	// Create a test request missing required parameters
	arguments := map[string]interface{}{
		"optional_string": "test_value",
		// Missing required_string and required_number
	}
	
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "test_tool",
			Arguments: arguments,
		},
	}
	
	// Execute the handler - this should work because the reflection system
	// handles missing parameters by using defaults or zero values
	ctx := context.Background()
	result, err := serverTool.Handler(ctx, request)
	
	// The handler should still execute (reflection handles missing params)
	if err != nil {
		t.Fatalf("Handler execution failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
}

// Test tool with array parameter
type TestToolWithArray struct {
	ToolInfo `name:"array_tool" description:"A test tool with array parameter"`
	
	Tags []string `json:"tags" description:"List of tags"`
}

func (t *TestToolWithArray) Handle(ctx context.Context) (interface{}, error) {
	return "array test result", nil
}

func TestReflectToolWithArrayParameter(t *testing.T) {
	// Create the server tool using reflection
	serverTool := ReflectTool[*TestToolWithArray]()
	
	// Verify the tool was created
	if serverTool.Tool.Name != "array_tool" {
		t.Errorf("Expected tool name 'array_tool', got '%s'", serverTool.Tool.Name)
	}
	
	// Verify the schema has the array property
	schema := serverTool.Tool.InputSchema
	
	// Check that properties exist
	if schema.Properties == nil {
		t.Fatal("Expected properties to be defined")
	}
	
	// Check that tags property exists
	if _, exists := schema.Properties["tags"]; !exists {
		t.Error("Expected tags property to exist")
	}
	
	// Verify we have 1 property
	if len(schema.Properties) != 1 {
		t.Errorf("Expected 1 property, got %d", len(schema.Properties))
	}
}

// Test tool with invalid description containing "default:"
type TestToolWithInvalidDescription struct {
	ToolInfo `name:"invalid_tool" description:"A test tool with invalid description"`
	
	BadField string `json:"bad_field" description:"A field with default: value in description"`
}

func (t *TestToolWithInvalidDescription) Handle(ctx context.Context) (interface{}, error) {
	return "should not reach here", nil
}

func TestReflectToolWithInvalidDescription(t *testing.T) {
	// This should panic during tool creation
	defer func() {
		if r := recover(); r != nil {
			// Check that the panic message mentions the validation error
			panicMsg := fmt.Sprintf("%v", r)
			if !strings.Contains(panicMsg, "description contains 'default:'") {
				t.Errorf("Expected panic about 'default:' in description, got: %s", panicMsg)
			}
			if !strings.Contains(panicMsg, "use separate 'default' struct tag") {
				t.Errorf("Expected panic to mention separate struct tag, got: %s", panicMsg)
			}
		} else {
			t.Error("Expected panic when description contains 'default:', but no panic occurred")
		}
	}()
	
	// This should panic
	ReflectTool[*TestToolWithInvalidDescription]()
}