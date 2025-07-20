package tmuxmcp

import (
	"fmt"
	"github.com/mark3labs/mcp-go/server"
	"log/slog"
	"time"
)

var Tools []server.ServerTool

func Run() error {
	version := fmt.Sprintf("1.0.%d", time.Now().UnixMilli())
	s := server.NewMCPServer("tmux", version, server.WithToolCapabilities(true))
	s.AddTools(Tools...)
	slog.Info("starting")
	return server.ServeStdio(s)
}
