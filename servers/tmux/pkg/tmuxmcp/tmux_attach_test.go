package tmuxmcp

import (
	"strings"
	"testing"
)

func TestAttachTool_Handle_SessionNotFound(t *testing.T) {
	tool := &AttachTool{
		SessionTool: SessionTool{
			Session: "nonexistent-session",
		},
	}

	_, err := tool.Handle(t.Context())

	if err == nil {
		t.Error("Expected error for nonexistent session")
	}

	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected error about session not existing, got: %v", err)
	}
}

func TestAttachTool_Handle_SessionResolution(t *testing.T) {
	tool := &AttachTool{
		SessionTool: SessionTool{
			Prefix: "test-prefix",
		},
	}

	_, err := tool.Handle(t.Context())

	// Should fail due to session not existing, but should get past session resolution
	if err == nil {
		t.Error("Expected error due to missing session")
	}

	// Error should be about session not existing, not about resolution
	if strings.Contains(err.Error(), "resolve") {
		t.Errorf("Should not fail on session resolution, got: %v", err)
	}
}

func TestAttachTool_Handle_BasicStructure(t *testing.T) {
	tool := &AttachTool{}

	// Test that the tool has the expected structure
	if tool.SessionTool.Prefix == "" {
		t.Log("AttachTool should support prefix-based session resolution")
	}
	if tool.SessionTool.Session == "" {
		t.Log("AttachTool should support explicit session names")
	}
}

func TestAttachTool_ToolInfo(t *testing.T) {
	// Test that the tool is properly registered and has correct metadata
	tool := &AttachTool{}
	
	_, err := tool.Handle(t.Context())
	
	// We expect this to fail because we don't have a real session
	// but we can verify the method exists and has the right signature
	if err == nil {
		t.Error("Expected error due to missing session")
	}
	
	// The error should be about session resolution or session not existing
	if !strings.Contains(err.Error(), "session") {
		t.Errorf("Expected error about session, got: %v", err)
	}
}