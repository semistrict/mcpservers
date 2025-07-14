package mcpcommon

import (
	"fmt"
	"github.com/mark3labs/mcp-go/server"
	"sort"
)

func PrintTools(tools []server.ServerTool) {
	// Sort tools by name for consistent output
	sortedTools := make([]server.ServerTool, len(tools))
	copy(sortedTools, tools)
	sort.Slice(sortedTools, func(i, j int) bool {
		return sortedTools[i].Tool.Name < sortedTools[j].Tool.Name
	})

	for _, serverTool := range sortedTools {
		tool := serverTool.Tool
		fmt.Printf("Tool: %s\n", tool.Name)
		if tool.Description != "" {
			fmt.Printf("  Description: %s\n", tool.Description)
		}

		// Print parameters
		if tool.InputSchema.Properties != nil {
			fmt.Printf("  Parameters:\n")

			// Sort properties for consistent output
			var propNames []string
			for name := range tool.InputSchema.Properties {
				propNames = append(propNames, name)
			}
			sort.Strings(propNames)

			for _, name := range propNames {
				prop := tool.InputSchema.Properties[name]
				required := false
				if tool.InputSchema.Required != nil {
					for _, req := range tool.InputSchema.Required {
						if req == name {
							required = true
							break
						}
					}
				}

				requiredStr := ""
				if required {
					requiredStr = " (required)"
				}

				// Try to extract type and description from the property (it's a map)
				typeStr := ""
				descStr := ""
				if propMap, ok := prop.(map[string]interface{}); ok {
					if propType, exists := propMap["type"]; exists {
						if typeVal, ok := propType.(string); ok {
							typeStr = fmt.Sprintf(" [%s]", typeVal)
						}
					}
					if propDesc, exists := propMap["description"]; exists {
						if descVal, ok := propDesc.(string); ok {
							descStr = descVal
						}
					}
				}

				fmt.Printf("    %s%s%s", name, typeStr, requiredStr)
				if descStr != "" {
					fmt.Printf(" - %s", descStr)
				}
				fmt.Println()
			}
		} else {
			fmt.Printf("  Parameters: none\n")
		}
		fmt.Println()
	}
}
