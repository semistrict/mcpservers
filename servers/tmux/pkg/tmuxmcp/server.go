package tmuxmcp

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/semistrict/mcpservers/pkg/mcpregistry"
	"github.com/semistrict/mcpservers/servers/tmux/pkg/tmux"
)

type Server struct {
	mcpregistry.MCPServerBase
	tmuxClient tmux.Client
}

var r mcpregistry.Registry[*Server]

// RegisterTool adds a tool registrar to the global registry
func RegisterTool(registrar mcpregistry.Registrar[*Server]) {
	r.Register(registrar)
}

func (s *Server) Run() error {
	s.MCPServer = server.NewMCPServer("tmux-mcp", "1.0.0")
	r.InitializeAll(s)
	return server.ServeStdio(s.MCPServer)
}
