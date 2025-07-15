package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"strings"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*ListTool]())
}

type ListTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_list" title:"List Tmux Sessions" description:"List all tmux sessions" destructive:"false" readonly:"true"`
	SessionTool
}

func (t *ListTool) Handle(ctx context.Context) (interface{}, error) {
	cmd := buildTmuxCommand( "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// No sessions exist
		sessions := []string{}
		if len(sessions) == 0 {
			result := "No tmux sessions found"
			if t.Prefix != "" {
				result += fmt.Sprintf(" with prefix '%s'", t.Prefix)
			}
			return result, nil
		}
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	if t.Prefix != "" {
		var filtered []string
		for _, session := range sessions {
			if strings.HasPrefix(session, t.Prefix) {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}

	if len(sessions) == 0 {
		result := "No tmux sessions found"
		if t.Prefix != "" {
			result += fmt.Sprintf(" with prefix '%s'", t.Prefix)
		}
		return result, nil
	}

	result := "Tmux sessions:\n"
	for _, session := range sessions {
		result += fmt.Sprintf("- %s\n", session)
	}

	return result, nil
}
