package mcpregistry

import "github.com/mark3labs/mcp-go/server"

// Registrar is a function that registers a tool with a server
// The generic type T should be the specific server type (e.g., *tmux.Server)
type Registrar[T HasMCPServer] func(T)

type HasMCPServer interface {
	GetMCPServer() *server.MCPServer
}

type MCPServerBase struct {
	*server.MCPServer
}

func (s MCPServerBase) GetMCPServer() *server.MCPServer {
	return s.MCPServer
}

// Registry holds tool registrars for a specific server type
type Registry[T HasMCPServer] struct {
	registrars []Registrar[T]
}

// Register adds a tool registrar to the registry
func (r *Registry[T]) Register(registrar Registrar[T]) {
	r.registrars = append(r.registrars, registrar)
}

// InitializeAll applies all registered tool registrars to the server
func (r *Registry[T]) InitializeAll(server T) {
	for _, registrar := range r.registrars {
		registrar(server)
	}
}
