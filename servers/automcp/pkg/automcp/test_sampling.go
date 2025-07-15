package automcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

type TestSamplingTool struct {
	mcpcommon.ToolInfo `name:"test_sampling" description:"Test MCP sampling functionality by requesting text generation"`

	Prompt       string  `json:"prompt" mcp:"required" description:"Prompt to send for text generation"`
	MaxTokens    int     `json:"max_tokens" description:"Maximum tokens to generate" default:"100"`
	Temperature  float64 `json:"temperature" description:"Temperature for sampling (0.0-1.0)" default:"0.7"`
	SystemPrompt string  `json:"system_prompt" description:"Optional system prompt" default:"You are a helpful assistant. Please respond concisely."`
}

func (t *TestSamplingTool) Handle(ctx context.Context) (interface{}, error) {
	// Get server from context
	serverFromCtx := server.ServerFromContext(ctx)
	if serverFromCtx == nil {
		return nil, fmt.Errorf("no server found in context")
	}

	if t.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// Set defaults
	if t.MaxTokens == 0 {
		t.MaxTokens = 100
	}
	if t.Temperature == 0 {
		t.Temperature = 0.7
	}
	if t.SystemPrompt == "" {
		t.SystemPrompt = "You are a helpful assistant. Please respond concisely."
	}

	// Create sampling request
	samplingRequest := mcp.CreateMessageRequest{
		CreateMessageParams: mcp.CreateMessageParams{
			Messages: []mcp.SamplingMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: t.Prompt,
					},
				},
			},
			ModelPreferences: &mcp.ModelPreferences{
				CostPriority:         0.5,
				SpeedPriority:        0.5,
				IntelligencePriority: 0.5,
			},
			SystemPrompt: t.SystemPrompt,
			MaxTokens:    t.MaxTokens,
			Temperature:  t.Temperature,
		},
	}

	// Request sampling
	result, err := serverFromCtx.RequestSampling(ctx, samplingRequest)
	if err != nil {
		return nil, fmt.Errorf("sampling request failed: %v", err)
	}

	// Format the response
	response := fmt.Sprintf("Sampling successful!\n\nModel: %s\nPrompt: %s\nMax Tokens: %d\nTemperature: %.1f\nSystem: %s\n\nResponse:\n%s",
		result.Model,
		t.Prompt,
		t.MaxTokens,
		t.Temperature,
		t.SystemPrompt,
		getTextFromContent(result.Content),
	)

	return response, nil
}

// Helper function to extract text from content
func getTextFromContent(content interface{}) string {
	switch c := content.(type) {
	case mcp.TextContent:
		return c.Text
	case map[string]interface{}:
		// Handle JSON unmarshaled content
		if text, ok := c["text"].(string); ok {
			return text
		}
		return fmt.Sprintf("%v", content)
	case string:
		return c
	default:
		return fmt.Sprintf("%v", content)
	}
}
