package tmuxmcp

import (
	"context"
	"fmt"
	"os"
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

	fmt.Printf("Test socket path: %s\n", testSocketPath)

	// Run tests
	code := m.Run()

	// Only clean up test socket if tests passed
	if code == 0 {
		// Teardown: Clean up test socket
		if err := cleanupTestSocket(); err != nil {
			fmt.Printf("Failed to cleanup test socket: %v\n", err)
			// Don't exit with error on cleanup failure
		}
	} else {
		fmt.Printf("Tests failed - preserving test socket at %s for debugging\n", testSocketPath)
	}

	os.Exit(code)
}

// setupTestSocket creates a test-specific tmux socket
func setupTestSocket() error {
	// Create a unique socket name for this test run
	// Use Unix timestamp and process ID for uniqueness
	timestamp := time.Now().Unix()
	pid := os.Getpid()
	testSocketPath = fmt.Sprintf("/tmp/tmux-test-%d-%d", timestamp, pid)

	return nil
}

// cleanupTestSocket removes the test socket and kills any test tmux sessions
func cleanupTestSocket() error {
	if testSocketPath == "" {
		return nil
	}

	// Kill any tmux sessions using our test socket
	ctx := context.Background()
	_, _ = runTmuxCommand(ctx, "kill-server") // Ignore errors - server might not be running

	// Remove the socket file if it exists
	if err := os.Remove(testSocketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove socket file: %w", err)
	}

	testSocketPath = ""

	return nil
}
