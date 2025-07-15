package automcp

import (
	"context"
	"testing"
)

func TestGeneratedToolHandle(t *testing.T) {
	// Create a simple tool definition for testing
	toolDef := ToolDefinition{
		Name:            "test_echo",
		Description:     "A test tool that echoes input",
		CommandTemplate: "echo hello",
		Parameters:      map[string]ParameterDef{},
	}

	// Create a GeneratedTool instance
	tool := &GeneratedTool{
		Definition:  toolDef,
		BaseCommand: "echo",
	}

	// Test the Handle method
	ctx := context.Background()
	result, err := tool.Handle(ctx)

	if err != nil {
		t.Fatalf("Handle() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Result should be a string containing the command output
	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}

	if resultStr == "" {
		t.Error("Expected non-empty result string")
	}
}

func TestGeneratedToolHandleWithArguments(t *testing.T) {
	// Create a tool definition with parameters
	toolDef := ToolDefinition{
		Name:            "test_echo_param",
		Description:     "A test tool that echoes parameter input",
		CommandTemplate: "echo {{.message}}",
		Parameters: map[string]ParameterDef{
			"message": {
				Type:        "string",
				Description: "Message to echo",
				Required:    true,
			},
		},
	}

	// Create a GeneratedTool instance
	tool := &GeneratedTool{
		Definition:  toolDef,
		BaseCommand: "echo",
	}

	// Test the HandleWithArguments method
	ctx := context.Background()
	arguments := map[string]interface{}{
		"message": "test message",
	}

	result, err := tool.HandleWithArguments(ctx, arguments)

	if err != nil {
		t.Fatalf("HandleWithArguments() failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Result should be a string containing the command output
	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}

	if resultStr == "" {
		t.Error("Expected non-empty result string")
	}
}

func TestGeneratedToolExecuteCommand(t *testing.T) {
	// Create a GeneratedTool instance
	tool := &GeneratedTool{
		Definition:  ToolDefinition{},
		BaseCommand: "echo",
	}

	// Test the ExecuteCommand method
	ctx := context.Background()
	output, err := tool.ExecuteCommand(ctx, "echo", []string{"direct test"})

	if err != nil {
		t.Fatalf("ExecuteCommand() failed: %v", err)
	}

	if output == "" {
		t.Error("Expected non-empty output")
	}

	// Output should contain our test message
	if output != "direct test\n" {
		t.Errorf("Expected 'direct test\\n', got %q", output)
	}
}

func TestGeneratedToolHandleWithMissingRequiredParameter(t *testing.T) {
	// Create a tool definition with required parameters
	toolDef := ToolDefinition{
		Name:            "test_required_param",
		Description:     "A test tool with required parameter",
		CommandTemplate: "echo {{.message}}",
		Parameters: map[string]ParameterDef{
			"message": {
				Type:        "string",
				Description: "Message to echo",
				Required:    true,
			},
		},
	}

	// Create a GeneratedTool instance
	tool := &GeneratedTool{
		Definition:  toolDef,
		BaseCommand: "echo",
	}

	// Test with missing required parameter
	ctx := context.Background()
	arguments := map[string]interface{}{} // Empty arguments

	result, err := tool.HandleWithArguments(ctx, arguments)

	// Should return an error for missing required parameter
	if err == nil {
		t.Error("Expected error for missing required parameter, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result on error, got %v", result)
	}
}

func TestGeneratedToolHandleWithDefaultParameter(t *testing.T) {
	// Create a tool definition with default parameter
	toolDef := ToolDefinition{
		Name:            "test_default_param",
		Description:     "A test tool with default parameter",
		CommandTemplate: "echo {{.message}}",
		Parameters: map[string]ParameterDef{
			"message": {
				Type:        "string",
				Description: "Message to echo",
				Required:    false,
				Default:     "default message",
			},
		},
	}

	// Create a GeneratedTool instance
	tool := &GeneratedTool{
		Definition:  toolDef,
		BaseCommand: "echo",
	}

	// Test with empty arguments (should use default)
	ctx := context.Background()
	arguments := map[string]interface{}{} // Empty arguments

	result, err := tool.HandleWithArguments(ctx, arguments)

	if err != nil {
		t.Fatalf("HandleWithArguments() with default failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Result should contain the default message
	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}

	if resultStr == "" {
		t.Error("Expected non-empty result string")
	}
}
