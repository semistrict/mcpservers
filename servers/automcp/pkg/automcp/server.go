package automcp

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
)

var Tools []server.ServerTool

func init() {
	Tools = []server.ServerTool{
		mcpcommon.ReflectTool[*TestSamplingTool](),
		mcpcommon.ReflectTool[*AnalyzeCliTool](),
	}
}

func Run() error {
	s := server.NewMCPServer("automcp", "1.0.0")

	// Enable sampling capability
	s.EnableSampling()

	s.AddTools(Tools...)
	return server.ServeStdio(s)
}
