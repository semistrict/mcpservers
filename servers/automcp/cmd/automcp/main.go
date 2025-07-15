package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"github.com/semistrict/mcpservers/servers/automcp/pkg/automcp"
)

func main() {
	help := flag.Bool("h", false, "Show help")
	flag.Parse()

	if *help {
		fmt.Println("automcp - MCP server for automation and testing")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  automcp       Start the MCP server (communicates via stdio)")
		fmt.Println("  automcp -h    Show this help message")
		fmt.Println()
		fmt.Println("Available tools:")
		fmt.Println()
		mcpcommon.PrintTools(automcp.Tools)
		return
	}

	if err := automcp.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
