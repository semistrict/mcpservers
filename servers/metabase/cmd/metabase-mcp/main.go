package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"github.com/semistrict/mcpservers/servers/metabase/pkg/metabasemcp"
)

func main() {
	var (
		help        bool
		serverURL   string
		cookiesFile string
	)

	flag.BoolVar(&help, "h", false, "Show available tools and their arguments")
	flag.StringVar(&serverURL, "server", "", "Metabase server URL (e.g., http://localhost:3000)")
	flag.StringVar(&cookiesFile, "cookies", "", "Path to cookies.txt file containing auth cookies (default: ~/.metabase/cookies.txt)")
	flag.Parse()

	if help {
		fmt.Println("metabase-mcp - MCP server for Metabase API")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  metabase-mcp -server URL -cookies FILE    Start the MCP server")
		fmt.Println("  metabase-mcp -h                           Show this help message")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -server URL    Metabase server URL (required)")
		fmt.Println("                 Can also be set via METABASE_API_URL environment variable")
		fmt.Println("  -cookies FILE  Path to cookies.txt file for authentication")
		fmt.Println("                 (default: ~/.metabase/cookies.txt)")
		fmt.Println()
		fmt.Println("Available tools:")
		fmt.Println()
		mcpcommon.PrintTools(metabasemcp.Tools)
		return
	}

	// Check environment variable if flag not provided
	if serverURL == "" {
		serverURL = os.Getenv("METABASE_API_URL")
	}

	if serverURL == "" {
		log.Fatal("Error: -server flag or METABASE_API_URL environment variable is required")
	}

	// Use default cookies file if not specified
	if cookiesFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting home directory: %v", err)
		}
		cookiesFile = fmt.Sprintf("%s/.metabase/cookies.txt", home)
	}

	if err := metabasemcp.Run(serverURL, cookiesFile); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
