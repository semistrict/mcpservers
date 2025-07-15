package automcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GeneratedTool wraps a ToolDefinition with execution capability
type GeneratedTool struct {
	Definition ToolDefinition
	BaseCommand string // The original CLI command (e.g., "docker", "git")
}


// ToolGenerator converts AI analysis results into executable MCP tools
type ToolGenerator struct {
	executor *SafeCommandExecutor
}

// NewToolGenerator creates a new tool generator
func NewToolGenerator() *ToolGenerator {
	return &ToolGenerator{
		executor: NewSafeCommandExecutor(),
	}
}

// GenerateServerTools converts ToolDefinitions into actual server.ServerTool instances
func (g *ToolGenerator) GenerateServerTools(analyses []CommandAnalysis) ([]server.ServerTool, error) {
	var serverTools []server.ServerTool
	
	for _, analysis := range analyses {
		for _, toolDef := range analysis.Tools {
			serverTool, err := g.createServerTool(toolDef, analysis.Command)
			if err != nil {
				return nil, fmt.Errorf("failed to create server tool %s: %v", toolDef.Name, err)
			}
			serverTools = append(serverTools, serverTool)
		}
	}
	
	return serverTools, nil
}

// createServerTool creates a single server.ServerTool from a ToolDefinition
func (g *ToolGenerator) createServerTool(toolDef ToolDefinition, baseCommand string) (server.ServerTool, error) {
	// Create MCP tool schema
	var options []mcp.ToolOption
	
	// Add description
	options = append(options, mcp.WithDescription(toolDef.Description))
	
	// Add parameters
	for paramName, param := range toolDef.Parameters {
		switch param.Type {
		case "string":
			var paramOptions []mcp.PropertyOption
			paramOptions = append(paramOptions, mcp.Description(param.Description))
			if !param.Required {
				if defaultVal, ok := param.Default.(string); ok {
					paramOptions = append(paramOptions, mcp.DefaultString(defaultVal))
				}
			}
			if param.Required {
				paramOptions = append(paramOptions, mcp.Required())
			}
			options = append(options, mcp.WithString(paramName, paramOptions...))
			
		case "number":
			var paramOptions []mcp.PropertyOption
			paramOptions = append(paramOptions, mcp.Description(param.Description))
			if !param.Required {
				if defaultVal, ok := param.Default.(float64); ok {
					paramOptions = append(paramOptions, mcp.DefaultNumber(defaultVal))
				}
			}
			if param.Required {
				paramOptions = append(paramOptions, mcp.Required())
			}
			options = append(options, mcp.WithNumber(paramName, paramOptions...))
			
		case "boolean":
			var paramOptions []mcp.PropertyOption
			paramOptions = append(paramOptions, mcp.Description(param.Description))
			if !param.Required {
				if defaultVal, ok := param.Default.(bool); ok {
					paramOptions = append(paramOptions, mcp.DefaultBool(defaultVal))
				}
			}
			options = append(options, mcp.WithBoolean(paramName, paramOptions...))
		}
	}
	
	// Create the MCP tool
	tool := mcp.NewTool(toolDef.Name, options...)
	
	// Create the handler function
	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return g.executeGeneratedTool(ctx, toolDef, baseCommand, request)
	}
	
	return server.ServerTool{
		Tool:    tool,
		Handler: handler,
	}, nil
}

// executeGeneratedTool executes a generated tool using the command template
func (g *ToolGenerator) executeGeneratedTool(ctx context.Context, toolDef ToolDefinition, baseCommand string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from request
	params := make(map[string]interface{})
	arguments := request.GetArguments()
	
	// Set parameter values from request, with defaults as fallback
	for paramName, paramDef := range toolDef.Parameters {
		if value, exists := arguments[paramName]; exists {
			params[paramName] = value
		} else if paramDef.Default != nil {
			params[paramName] = paramDef.Default
		} else if paramDef.Required {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Required parameter '%s' is missing", paramName),
					},
				},
				IsError: true,
			}, nil
		}
	}
	
	// Execute the command template
	command, err := ExecuteCommandTemplate(toolDef.CommandTemplate, params)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Template execution failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}
	
	// Parse the command into parts
	commandParts := strings.Fields(command)
	if len(commandParts) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Generated command is empty",
				},
			},
			IsError: true,
		}, nil
	}
	
	// Execute the command safely
	mainCommand := commandParts[0]
	args := commandParts[1:]
	
	output, err := g.executor.ExecuteCommand(mainCommand, args)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Command execution failed: %v\n\nCommand: %s\nOutput: %s", err, command, string(output)),
				},
			},
			IsError: true,
		}, nil
	}
	
	// Return successful result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Command: %s\n\nOutput:\n%s", command, string(output)),
			},
		},
	}, nil
}

// ValidateToolDefinition checks if a tool definition is valid for generation
func (g *ToolGenerator) ValidateToolDefinition(toolDef ToolDefinition) error {
	if toolDef.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	
	if toolDef.Description == "" {
		return fmt.Errorf("tool description is required")
	}
	
	if toolDef.CommandTemplate == "" {
		return fmt.Errorf("command template is required")
	}
	
	// Validate template syntax
	testParams := make(map[string]interface{})
	for paramName, param := range toolDef.Parameters {
		switch param.Type {
		case "string":
			testParams[paramName] = "test"
		case "number":
			testParams[paramName] = 1
		case "boolean":
			testParams[paramName] = true
		}
	}
	
	_, err := ExecuteCommandTemplate(toolDef.CommandTemplate, testParams)
	if err != nil {
		return fmt.Errorf("invalid command template: %v", err)
	}
	
	return nil
}