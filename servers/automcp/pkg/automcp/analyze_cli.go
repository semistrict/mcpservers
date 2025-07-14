package automcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

type AnalyzeCliTool struct {
	mcpcommon.ToolInfo `name:"analyze_cli" description:"Analyze a CLI command's help output and generate MCP tool definitions"`
	
	Command     string   `json:"command,required" description:"CLI command to analyze (e.g., 'docker', 'git', 'kubectl')"`
	HelpFlags   []string `json:"help_flags" description:"Help flags to try"`
	Subcommand  string   `json:"subcommand" description:"Optional subcommand to analyze (e.g., 'docker build')"`
	MaxTokens   int      `json:"max_tokens" description:"Maximum tokens for AI analysis" default:"4000"`
	Temperature float64  `json:"temperature" description:"Temperature for AI analysis" default:"0.3"`
	MaxDepth    int      `json:"max_depth" description:"Maximum recursion depth for subcommands" default:"3"`
	MaxRequests int      `json:"max_requests" description:"Maximum total sampling requests" default:"20"`
	Recursive   bool     `json:"recursive" description:"Recursively analyze all subcommands" default:"true"`
}

// AIResponse represents the expected structure from the AI
type AIResponse struct {
	Tools        []ToolDefinition `json:"tools"`
	Summary      string           `json:"summary"`
	IsLeaf       bool             `json:"is_leaf"`
	Subcommands  []string         `json:"subcommands,omitempty"`
}

// CommandAnalysis tracks the analysis of a command tree
type CommandAnalysis struct {
	Command     string
	Subcommand  string
	FullPath    string
	HelpOutput  string
	Tools       []ToolDefinition
	Subcommands []string
	IsLeaf      bool
	Depth       int
}

// ToolDefinition represents a single MCP tool definition
type ToolDefinition struct {
	Name            string                    `json:"name"`
	Description     string                    `json:"description"`
	Parameters      map[string]ParameterDef   `json:"parameters"`
	CommandTemplate string                    `json:"command_template"`
}

// ParameterDef represents a parameter definition
type ParameterDef struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

func (t *AnalyzeCliTool) Handle(ctx context.Context) (interface{}, error) {
	if t.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Set defaults
	if len(t.HelpFlags) == 0 {
		t.HelpFlags = GetSafeHelpFlags()
	}
	if t.MaxTokens == 0 {
		t.MaxTokens = 4000
	}
	if t.Temperature == 0 {
		t.Temperature = 0.3
	}
	if t.MaxDepth == 0 {
		t.MaxDepth = 3
	}
	if t.MaxRequests == 0 {
		t.MaxRequests = 20
	}

	// Analyze the command tree recursively
	var allAnalyses []CommandAnalysis
	var allWarnings []string

	if t.Recursive {
		analyses, warnings, err := t.analyzeCommandTree(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze command tree: %v", err)
		}
		allAnalyses = analyses
		allWarnings = warnings
	} else {
		// Single command analysis (legacy behavior)
		analysis, warnings, err := t.analyzeSingleCommand(ctx, t.Subcommand, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze command: %v", err)
		}
		allAnalyses = []CommandAnalysis{*analysis}
		allWarnings = warnings
	}

	// Format the results
	return t.formatResults(allAnalyses, allWarnings), nil
}

func (t *AnalyzeCliTool) analyzeCommandTree(ctx context.Context) ([]CommandAnalysis, []string, error) {
	var allAnalyses []CommandAnalysis
	var allWarnings []string
	requestCount := 0
	
	// Start with the root command
	rootAnalysis, warnings, err := t.analyzeSingleCommand(ctx, t.Subcommand, 0)
	if err != nil {
		return nil, warnings, err
	}
	
	requestCount++
	allAnalyses = append(allAnalyses, *rootAnalysis)
	allWarnings = append(allWarnings, warnings...)
	
	// Queue for breadth-first traversal
	type queueItem struct {
		subcommand string
		depth      int
	}
	
	queue := make([]queueItem, 0)
	for _, subcmd := range rootAnalysis.Subcommands {
		queue = append(queue, queueItem{subcmd, 1})
	}
	
	// Process all subcommands
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		
		// Skip if we've reached max depth
		if item.depth >= t.MaxDepth {
			allWarnings = append(allWarnings, fmt.Sprintf("Skipping '%s': reached max depth %d", item.subcommand, t.MaxDepth))
			continue
		}
		
		// Skip if we've reached max requests
		if requestCount >= t.MaxRequests {
			allWarnings = append(allWarnings, fmt.Sprintf("Stopping analysis: reached max requests limit (%d). %d commands remain in queue", t.MaxRequests, len(queue)+1))
			break
		}
		
		analysis, warnings, err := t.analyzeSingleCommand(ctx, item.subcommand, item.depth)
		requestCount++
		
		if err != nil {
			allWarnings = append(allWarnings, fmt.Sprintf("Failed to analyze '%s': %v", item.subcommand, err))
			continue
		}
		
		allAnalyses = append(allAnalyses, *analysis)
		allWarnings = append(allWarnings, warnings...)
		
		// Add subcommands to queue
		for _, subcmd := range analysis.Subcommands {
			fullSubcmd := item.subcommand + " " + subcmd
			queue = append(queue, queueItem{fullSubcmd, item.depth + 1})
		}
	}
	
	// Add summary of request usage
	allWarnings = append(allWarnings, fmt.Sprintf("Total sampling requests used: %d/%d", requestCount, t.MaxRequests))
	
	return allAnalyses, allWarnings, nil
}

func (t *AnalyzeCliTool) analyzeSingleCommand(ctx context.Context, subcommand string, depth int) (*CommandAnalysis, []string, error) {
	// Get help output for this specific command
	helpOutput, err := t.getHelpOutputForSubcommand(subcommand)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get help output: %v", err)
	}
	
	// Analyze with AI
	aiResponse, warnings := t.analyzeWithAI(ctx, helpOutput, subcommand, depth)
	if aiResponse == nil {
		// Include warnings in error for debugging
		warningText := ""
		if len(warnings) > 0 {
			warningText = fmt.Sprintf(" Warnings: %v", warnings)
		}
		return nil, warnings, fmt.Errorf("AI analysis failed for command '%s'.%s", subcommand, warningText)
	}
	
	fullPath := t.Command
	if subcommand != "" {
		fullPath = t.Command + " " + subcommand
	}
	
	analysis := &CommandAnalysis{
		Command:     t.Command,
		Subcommand:  subcommand,
		FullPath:    fullPath,
		HelpOutput:  helpOutput,
		Tools:       aiResponse.Tools,
		Subcommands: aiResponse.Subcommands,
		IsLeaf:      aiResponse.IsLeaf,
		Depth:       depth,
	}
	
	return analysis, warnings, nil
}

func (t *AnalyzeCliTool) getHelpOutputForSubcommand(subcommand string) (string, error) {
	// Check if command is safe to execute
	if !IsCommandSafe(t.Command) {
		return "", fmt.Errorf("command '%s' is blocked for security reasons", t.Command)
	}
	
	var cmdArgs []string
	
	// Build command with subcommand
	if subcommand != "" {
		cmdArgs = append(cmdArgs, strings.Fields(subcommand)...)
	}

	// Create safe executor
	executor := NewSafeCommandExecutor()

	// Try different help flags
	var lastErr error
	for _, helpFlag := range t.HelpFlags {
		args := append(cmdArgs, helpFlag)
		
		output, err := executor.ExecuteCommand(t.Command, args)
		if err == nil && len(output) > 0 {
			return string(output), nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("failed to get help output after trying flags %v: %v", t.HelpFlags, lastErr)
}

func (t *AnalyzeCliTool) getHelpOutput() (string, error) {
	return t.getHelpOutputForSubcommand(t.Subcommand)
}

func (t *AnalyzeCliTool) analyzeWithAI(ctx context.Context, helpOutput, subcommand string, depth int) (*AIResponse, []string) {
	serverFromCtx := server.ServerFromContext(ctx)
	if serverFromCtx == nil {
		return nil, []string{"no server found in context"}
	}

	commandDesc := t.Command
	if subcommand != "" {
		commandDesc = fmt.Sprintf("%s %s", t.Command, subcommand)
	}

	prompt := fmt.Sprintf(`Analyze this CLI help output and respond with ONLY valid JSON. No explanations, no markdown, no other text.

Command: %s
Help Output:
%s

Respond with valid JSON in this exact format:
{
  "is_leaf": true,
  "subcommands": null,
  "tools": [
    {
      "name": "tool_name",
      "description": "What this tool does",
      "parameters": {
        "param_name": {
          "type": "string",
          "description": "Parameter description",
          "required": false,
          "default": "default_value"
        }
      },
      "command_template": "command {{.param_name}}"
    }
  ],
  "summary": "Brief summary"
}

Rules:
- If command has subcommands: set is_leaf=false, list subcommands, empty tools array
- If leaf command: set is_leaf=true, null subcommands, create tools
- Only include most useful parameters (max 5 per tool)
- Use only these parameter types: string, number, boolean
- Use Go text/template syntax in command_template: {{.param_name}}
- For optional flags use: {{if .flag}}-flag {{.flag}}{{end}}
- For boolean flags use: {{if .verbose}}-v{{end}}
- Example: "ls {{if .all}}-a{{end}} {{if .long}}-l{{end}} {{.path}}"
- RESPOND ONLY WITH VALID JSON`, commandDesc, helpOutput)

	samplingRequest := mcp.CreateMessageRequest{
		CreateMessageParams: mcp.CreateMessageParams{
			Messages: []mcp.SamplingMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: prompt,
					},
				},
			},
			SystemPrompt: "You are a CLI analysis expert. You MUST respond with ONLY valid, complete JSON. Never include explanations, markdown, or any text outside the JSON object. Use Go text/template syntax ({{.param_name}}) in command_template fields. Ensure all JSON is properly closed with matching braces and brackets.",
			MaxTokens:    t.MaxTokens,
			Temperature:  t.Temperature,
		},
	}

	result, err := serverFromCtx.RequestSampling(ctx, samplingRequest)
	if err != nil {
		return nil, []string{fmt.Sprintf("sampling request failed: %v", err)}
	}

	aiResponseText := getTextFromContent(result.Content)
	
	// Add debug info to warnings
	extractedJSON := t.extractJSON(aiResponseText)
	debugWarnings := []string{
		fmt.Sprintf("AI response length: %d characters", len(aiResponseText)),
		fmt.Sprintf("Extracted JSON length: %d characters", len(extractedJSON)),
		fmt.Sprintf("Full extracted JSON: %s", extractedJSON),
	}
	
	// Validate and parse the AI response
	aiResponse, validationErrors := t.validateAIResponse(aiResponseText)
	
	// Combine debug and validation warnings
	allWarnings := append(debugWarnings, validationErrors...)
	
	return aiResponse, allWarnings
}

func (t *AnalyzeCliTool) formatResults(analyses []CommandAnalysis, warnings []string) string {
	var response strings.Builder
	
	response.WriteString(fmt.Sprintf("# CLI Analysis Tree for: %s\n\n", t.Command))
	
	if len(warnings) > 0 {
		response.WriteString("## Validation Warnings\n")
		for _, warning := range warnings {
			response.WriteString(fmt.Sprintf("⚠️  %s\n", warning))
		}
		response.WriteString("\n")
	}
	
	// Summary statistics
	totalTools := 0
	leafCommands := 0
	for _, analysis := range analyses {
		totalTools += len(analysis.Tools)
		if analysis.IsLeaf {
			leafCommands++
		}
	}
	
	response.WriteString(fmt.Sprintf("## Summary\n"))
	response.WriteString(fmt.Sprintf("- **Total Commands Analyzed:** %d\n", len(analyses)))
	response.WriteString(fmt.Sprintf("- **Leaf Commands:** %d\n", leafCommands))
	response.WriteString(fmt.Sprintf("- **Total Tools Generated:** %d\n\n", totalTools))
	
	// Command tree structure
	response.WriteString("## Command Tree\n")
	for _, analysis := range analyses {
		indent := strings.Repeat("  ", analysis.Depth)
		if analysis.IsLeaf {
			response.WriteString(fmt.Sprintf("%s🔧 %s (%d tools)\n", indent, analysis.FullPath, len(analysis.Tools)))
		} else {
			response.WriteString(fmt.Sprintf("%s📁 %s (%d subcommands)\n", indent, analysis.FullPath, len(analysis.Subcommands)))
		}
	}
	response.WriteString("\n")
	
	// All generated tools
	response.WriteString("## Generated MCP Tools\n\n")
	for _, analysis := range analyses {
		if len(analysis.Tools) > 0 {
			response.WriteString(fmt.Sprintf("### %s\n\n", analysis.FullPath))
			
			toolsJSON, err := json.MarshalIndent(analysis.Tools, "", "  ")
			if err == nil {
				response.WriteString(fmt.Sprintf("```json\n%s\n```\n\n", string(toolsJSON)))
			}
			
			// Add template execution examples
			response.WriteString("**Template Examples:**\n\n")
			for _, tool := range analysis.Tools {
				response.WriteString(fmt.Sprintf("- **%s**: `%s`\n", tool.Name, tool.CommandTemplate))
				
				// Show example execution with default values
				exampleParams := make(map[string]interface{})
				for paramName, param := range tool.Parameters {
					if param.Default != nil {
						exampleParams[paramName] = param.Default
					} else {
						// Provide example values based on type
						switch param.Type {
						case "string":
							exampleParams[paramName] = "example"
						case "boolean":
							exampleParams[paramName] = true
						case "number":
							exampleParams[paramName] = 1
						}
					}
				}
				
				if len(exampleParams) > 0 {
					if execResult, err := ExecuteCommandTemplate(tool.CommandTemplate, exampleParams); err == nil {
						response.WriteString(fmt.Sprintf("  - Example: `%s`\n", execResult))
					}
				}
			}
			response.WriteString("\n")
		}
	}
	
	return response.String()
}


func (t *AnalyzeCliTool) validateAIResponse(responseText string) (*AIResponse, []string) {
	var warnings []string
	
	// Try to extract JSON from the response (in case it's wrapped in markdown)
	jsonText := t.extractJSON(responseText)
	if jsonText == "" {
		warnings = append(warnings, "No JSON found in AI response")
		// Return a fallback response instead of nil
		return &AIResponse{
			Tools:       []ToolDefinition{},
			Summary:     "Failed to parse AI response",
			IsLeaf:      true,
			Subcommands: []string{},
		}, warnings
	}
	
	
	// Parse JSON
	var aiResponse AIResponse
	if err := json.Unmarshal([]byte(jsonText), &aiResponse); err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to parse JSON: %v", err))
		// Return a fallback response instead of nil
		return &AIResponse{
			Tools:       []ToolDefinition{},
			Summary:     fmt.Sprintf("JSON parse error: %v", err),
			IsLeaf:      true,
			Subcommands: []string{},
		}, warnings
	}
	
	// Validate structure - but don't fail completely
	if len(aiResponse.Tools) == 0 && len(aiResponse.Subcommands) == 0 {
		warnings = append(warnings, "No tools or subcommands found in response")
		aiResponse.IsLeaf = true // Default to leaf if unclear
	}
	
	// Validate each tool
	validTools := make([]ToolDefinition, 0, len(aiResponse.Tools))
	for i, tool := range aiResponse.Tools {
		toolWarnings := t.validateTool(tool, i)
		warnings = append(warnings, toolWarnings...)
		
		// Clean up tool name
		tool.Name = t.sanitizeToolName(tool.Name)
		
		// Only include valid tools
		if tool.Name != "" && tool.Description != "" && tool.CommandTemplate != "" {
			validTools = append(validTools, tool)
		}
	}
	
	aiResponse.Tools = validTools
	
	if len(validTools) == 0 {
		warnings = append(warnings, "No valid tools after validation")
	}
	
	return &aiResponse, warnings
}

func (t *AnalyzeCliTool) extractJSON(text string) string {
	// Since we're asking for pure JSON, just trim whitespace
	trimmed := strings.TrimSpace(text)
	
	// If it starts and ends with braces, return as-is
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return trimmed
	}
	
	// Try to find the first { and last } in the text as fallback
	firstBrace := strings.Index(text, "{")
	lastBrace := strings.LastIndex(text, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		return strings.TrimSpace(text[firstBrace : lastBrace+1])
	}
	
	// Return original text if no JSON structure found
	return trimmed
}


func (t *AnalyzeCliTool) validateTool(tool ToolDefinition, index int) []string {
	var warnings []string
	
	if tool.Name == "" {
		warnings = append(warnings, fmt.Sprintf("Tool %d: missing name", index))
	}
	
	if tool.Description == "" {
		warnings = append(warnings, fmt.Sprintf("Tool %d (%s): missing description", index, tool.Name))
	}
	
	if tool.CommandTemplate == "" {
		warnings = append(warnings, fmt.Sprintf("Tool %d (%s): missing command template", index, tool.Name))
	}
	
	// Validate parameter types
	for paramName, param := range tool.Parameters {
		if param.Type == "" {
			warnings = append(warnings, fmt.Sprintf("Tool %d (%s): parameter '%s' missing type", index, tool.Name, paramName))
		} else if !t.isValidParameterType(param.Type) {
			warnings = append(warnings, fmt.Sprintf("Tool %d (%s): parameter '%s' has invalid type '%s'", index, tool.Name, paramName, param.Type))
		}
		
		if param.Description == "" {
			warnings = append(warnings, fmt.Sprintf("Tool %d (%s): parameter '%s' missing description", index, tool.Name, paramName))
		}
	}
	
	return warnings
}

func (t *AnalyzeCliTool) sanitizeToolName(name string) string {
	// Remove invalid characters and convert to lowercase with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	sanitized := reg.ReplaceAllString(name, "_")
	sanitized = strings.ToLower(sanitized)
	sanitized = strings.Trim(sanitized, "_")
	
	// Ensure it doesn't start with a number
	if len(sanitized) > 0 && sanitized[0] >= '0' && sanitized[0] <= '9' {
		sanitized = "cmd_" + sanitized
	}
	
	return sanitized
}

func (t *AnalyzeCliTool) isValidParameterType(paramType string) bool {
	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"integer": true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}
	return validTypes[paramType]
}

// ExecuteCommandTemplate executes a Go text/template command template with given parameters
func ExecuteCommandTemplate(commandTemplate string, params map[string]interface{}) (string, error) {
	tmpl, err := template.New("command").Parse(commandTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}
	
	// Clean up extra whitespace
	command := strings.Join(strings.Fields(buf.String()), " ")
	return command, nil
}