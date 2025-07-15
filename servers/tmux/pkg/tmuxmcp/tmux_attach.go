package tmuxmcp

import (
	"context"
	"fmt"
	"github.com/semistrict/mcpservers/pkg/mcpcommon"
	"os/exec"
	"runtime"
)

func init() {
	Tools = append(Tools, mcpcommon.ReflectTool[*AttachTool]())
}

type AttachTool struct {
	_ mcpcommon.ToolInfo `name:"tmux_attach" title:"Attach to Tmux Session" description:"Open tmux session in terminal program (iTerm2 on macOS, gnome-terminal on Linux)" destructive:"false" readonly:"true"`
	SessionTool
}

func (t *AttachTool) Handle(ctx context.Context) (interface{}, error) {
	sessionName, err := resolveSession(ctx, t.Prefix, t.Session)
	if err != nil {
		return nil, fmt.Errorf("error attaching to session: %v", err)
	}

	// Check if session exists
	if !sessionExists(ctx, sessionName) {
		return nil, fmt.Errorf("session %s does not exist", sessionName)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// macOS - use iTerm2 if available, fall back to Terminal.app
		// First try iTerm2
		cmd = exec.Command("osascript", "-e", fmt.Sprintf(`
			tell application "iTerm2"
				activate
				tell current window
					create tab with default profile
					tell current session
						write text "tmux attach-session -t %s"
					end tell
				end tell
			end tell
		`, sessionName))
		
		err = cmd.Run()
		if err != nil {
			// Fall back to Terminal.app
			cmd = exec.Command("osascript", "-e", fmt.Sprintf(`
				tell application "Terminal"
					activate
					do script "tmux attach-session -t %s"
				end tell
			`, sessionName))
			err = cmd.Run()
		}

	case "linux":
		// Linux - try common terminal emulators
		terminals := [][]string{
			{"gnome-terminal", "--", "tmux", "attach-session", "-t", sessionName},
			{"konsole", "-e", "tmux", "attach-session", "-t", sessionName},
			{"xterm", "-e", "tmux", "attach-session", "-t", sessionName},
		}
		
		var lastErr error
		for _, termCmd := range terminals {
			cmd = exec.Command(termCmd[0], termCmd[1:]...)
			err = cmd.Start()
			if err == nil {
				break
			}
			lastErr = err
		}
		if err != nil {
			err = lastErr
		}

	case "windows":
		// Windows - use Windows Terminal if available, fall back to cmd
		cmd = exec.Command("wt", "tmux", "attach-session", "-t", sessionName)
		err = cmd.Start()
		if err != nil {
			// Fall back to cmd
			cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", "tmux", "attach-session", "-t", sessionName)
			err = cmd.Start()
		}

	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open terminal for session %s: %w", sessionName, err)
	}

	return fmt.Sprintf("Opening session %s in terminal program", sessionName), nil
}