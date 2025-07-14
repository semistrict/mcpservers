package main

import (
	"flag"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"log"
	"os"

	"github.com/semistrict/mcpservers/servers/tmux/pkg/tmuxmcp"
)

func main() {
	var help bool
	flag.BoolVar(&help, "h", false, "Show available tools and their arguments")
	flag.Parse()

	if help {
		fmt.Println("tmux-mcp - MCP server for tmux session management")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  tmux-mcp       Start the MCP server (communicates via stdio)")
		fmt.Println("  tmux-mcp -h    Show this help message")
		fmt.Println()
		fmt.Println("Available tools:")
		fmt.Println()
		mcpcommon.PrintTools(tmuxmcp.Tools)
		return
	}

	if err := tmuxmcp.Run(); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
