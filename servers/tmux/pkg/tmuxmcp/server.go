package tmuxmcp

import (
	"github.com/mark3labs/mcp-go/server"
	"log/slog"
)

var Tools []server.ServerTool

func Run() error {
	s := server.NewMCPServer("tmux-mcp", "1.0.0", server.WithToolCapabilities(true))
	s.AddTools(Tools...)
	slog.Info("starting")
	return server.ServeStdio(s)
}
