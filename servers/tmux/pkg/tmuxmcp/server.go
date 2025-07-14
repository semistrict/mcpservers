package tmuxmcp

import (
	"github.com/mark3labs/mcp-go/server"
)

var Tools []server.ServerTool

func Run() error {
	s := server.NewMCPServer("tmux-mcp", "1.0.0")
	s.AddTools(Tools...)
	return server.ServeStdio(s)
}
