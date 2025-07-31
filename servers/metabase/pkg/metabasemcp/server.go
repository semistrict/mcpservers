package metabasemcp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

var Tools []server.ServerTool

func Run(serverURL, cookiesFile string) error {
	// Initialize the client with the provided configuration
	if err := InitializeClient(serverURL, cookiesFile); err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	version := fmt.Sprintf("1.0.%d", time.Now().UnixMilli())
	s := server.NewMCPServer("metabase", version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false), // subscribe=true, listChanged=false
	)
	s.AddTools(Tools...)

	// Start async database resource loading
	go loadDatabaseResources(context.Background(), s.AddResource)

	slog.Info("starting", "server", serverURL)
	return server.ServeStdio(s)
}
