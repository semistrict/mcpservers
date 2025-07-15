package tmuxmcp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// createUniqueSession creates a new tmux session with a unique name
// It will try incrementing numbers until it finds an available session name
func createUniqueSession(prefix string, command []string) (string, error) {
	if prefix == "" {
		prefix = detectPrefix()
	}
	
	// Generate base name from command
	var cmdPart string
	if len(command) > 0 {
		cmdBase := filepath.Base(command[0])
		if len(cmdBase) > 10 {
			cmdBase = cmdBase[:10]
		}
		reg := regexp.MustCompile(`[^a-zA-Z0-9]`)
		cmdPart = reg.ReplaceAllString(cmdBase, "")
		if cmdPart == "" {
			cmdPart = "session"
		}
	} else {
		cmdPart = "session"
	}
	
	// Get existing sessions with this prefix
	existingSessions, err := list("")
	if err != nil {
		// If we can't list sessions, assume none exist
		existingSessions = []string{}
	}
	
	// Find the highest number used for this prefix-cmdPart combination
	baseName := fmt.Sprintf("%s-%s", prefix, cmdPart)
	maxNumber := 0
	
	for _, session := range existingSessions {
		if strings.HasPrefix(session, baseName+"-") {
			// Extract number from session name like "prefix-cmdPart-123"
			parts := strings.Split(session, "-")
			if len(parts) >= 3 {
				if num, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
					if num > maxNumber {
						maxNumber = num
					}
				}
			}
		}
	}
	
	// Try creating sessions with incrementing numbers
	for i := maxNumber + 1; i < maxNumber + 100; i++ { // Try up to 100 attempts
		sessionName := fmt.Sprintf("%s-%d", baseName, i)
		
		// Try to create the session
		var cmd *exec.Cmd
		if len(command) > 0 {
			args := append([]string{"new-session", "-d", "-s", sessionName}, command...)
			cmd = buildTmuxCommand(args...)
		} else {
			cmd = buildTmuxCommand("new-session", "-d", "-s", sessionName)
		}
		
		if err := cmd.Run(); err == nil {
			// Success! Return the session name
			return sessionName, nil
		}
		// If failed, try next number
	}
	
	return "", fmt.Errorf("failed to create unique session after 100 attempts")
}

// openSessionInTerminal opens a tmux session in read-only mode in the user's terminal
func openSessionInTerminal(sessionName string) error {
	// Get the user's terminal program
	terminalProgram := os.Getenv("TERMINAL_PROGRAM")
	if terminalProgram == "" {
		// Default to iTerm if not set
		terminalProgram = "iTerm.app"
	}
	
	// Build the tmux command to attach in read-only mode
	var tmuxCmd string
	if testSocketPath != "" {
		// In test mode, use the test socket
		tmuxCmd = fmt.Sprintf("tmux -S %s attach-session -t %s -r", testSocketPath, sessionName)
	} else {
		// Normal mode
		tmuxCmd = fmt.Sprintf("tmux attach-session -t %s -r", sessionName)
	}
	
	// Different terminal programs require different approaches
	switch terminalProgram {
	case "iTerm.app":
		// Use AppleScript to open a new iTerm window with the tmux session
		appleScript := fmt.Sprintf(`
			tell application "iTerm"
				create window with default profile
				tell current session of current window
					write text "%s"
				end tell
			end tell
		`, tmuxCmd)
		
		cmd := exec.Command("osascript", "-e", appleScript)
		return cmd.Run()
		
	case "Terminal.app":
		// Use AppleScript for Terminal.app
		appleScript := fmt.Sprintf(`
			tell application "Terminal"
				do script "%s"
			end tell
		`, tmuxCmd)
		
		cmd := exec.Command("osascript", "-e", appleScript)
		return cmd.Run()
		
	default:
		// For unknown terminals, try using the 'open' command
		// This works for many terminal apps on macOS
		cmd := exec.Command("open", "-a", terminalProgram, "--args", "--", tmuxCmd)
		return cmd.Run()
	}
}