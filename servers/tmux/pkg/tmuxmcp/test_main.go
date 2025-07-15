package tmuxmcp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMain provides setup and teardown for all tests
func TestMain(m *testing.M) {
	// Setup: Create test-specific socket
	if err := setupTestSocket(); err != nil {
		fmt.Printf("Failed to setup test socket: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown: Clean up test socket
	if err := cleanupTestSocket(); err != nil {
		fmt.Printf("Failed to cleanup test socket: %v\n", err)
		// Don't exit with error on cleanup failure
	}

	os.Exit(code)
}

// setupTestSocket creates a test-specific tmux socket
func setupTestSocket() error {
	// Create a unique socket name for this test run
	timestamp := time.Now().UnixNano()
	socketName := fmt.Sprintf("tmuxmcp-test-%d", timestamp)
	
	// Create temp directory for socket
	tmpDir, err := os.MkdirTemp("", "tmuxmcp-test-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	
	testSocketPath = filepath.Join(tmpDir, socketName)
	
	return nil
}

// cleanupTestSocket removes the test socket and kills any test tmux sessions
func cleanupTestSocket() error {
	if testSocketPath == "" {
		return nil
	}
	
	// Kill any tmux sessions using our test socket
	cmd := buildTmuxCommand("kill-server")
	cmd.Run() // Ignore errors - server might not be running
	
	// Remove the socket directory
	socketDir := filepath.Dir(testSocketPath)
	if err := os.RemoveAll(socketDir); err != nil {
		return fmt.Errorf("failed to remove socket directory: %w", err)
	}
	
	testSocketPath = ""
	
	return nil
}