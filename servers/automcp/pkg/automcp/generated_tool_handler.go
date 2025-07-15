package automcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handle implements the ToolHandler interface for generated tools
func (g *GeneratedTool) Handle(ctx context.Context) (interface{}, error) {
	// Create a ToolGenerator instance to execute the tool
	generator := NewToolGenerator()

	// Create a mock request with empty arguments since we don't have them in this context
	// This is a limitation of the current interface - we need the request arguments
	// to properly execute the tool
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      g.Definition.Name,
			Arguments: make(map[string]interface{}),
		},
	}

	// Execute the tool using the existing implementation
	result, err := generator.executeGeneratedTool(ctx, g.Definition, g.BaseCommand, request)
	if err != nil {
		return nil, err
	}

	// Convert the MCP result to a simple interface for the ToolHandler
	if result.IsError {
		// Extract error message from content
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				return nil, fmt.Errorf("%s", textContent.Text)
			}
		}
		return nil, fmt.Errorf("tool execution failed")
	}

	// Extract text content from successful result
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "Tool executed successfully", nil
}

// HandleWithArguments provides a more complete implementation that accepts arguments
func (g *GeneratedTool) HandleWithArguments(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	// Create a ToolGenerator instance to execute the tool
	generator := NewToolGenerator()

	// Create a proper request with the provided arguments
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      g.Definition.Name,
			Arguments: arguments,
		},
	}

	// Execute the tool using the existing implementation
	result, err := generator.executeGeneratedTool(ctx, g.Definition, g.BaseCommand, request)
	if err != nil {
		return nil, err
	}

	// Convert the MCP result to a simple interface for the ToolHandler
	if result.IsError {
		// Extract error message from content
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				return nil, fmt.Errorf("%s", textContent.Text)
			}
		}
		return nil, fmt.Errorf("tool execution failed")
	}

	// Extract text content from successful result
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	return "Tool executed successfully", nil
}

// ExecuteCommand is a convenience method that executes a command directly
func (g *GeneratedTool) ExecuteCommand(ctx context.Context, command string, args []string) (string, error) {
	// Create a safe command executor
	executor := NewSafeCommandExecutor()

	// Execute the command
	output, err := executor.ExecuteCommand(command, args)
	if err != nil {
		return "", fmt.Errorf("command execution failed: %v\n\nCommand: %s %s\nOutput: %s",
			err, command, strings.Join(args, " "), string(output))
	}

	return string(output), nil
}
