package automcp

import (
	"testing"
)

func TestSafeCommandExecutor_ExecuteCommand_Safe(t *testing.T) {
	executor := NewSafeCommandExecutor()
	output, err := executor.ExecuteCommand("echo", []string{"hello world"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if string(output) == "" || string(output) != "hello world\n" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestSafeCommandExecutor_ExecuteCommand_Dangerous(t *testing.T) {
	executor := NewSafeCommandExecutor()
	_, err := executor.ExecuteCommand("reboot", []string{})
	if err == nil {
		t.Error("expected error for dangerous command, got nil")
	}
}

func TestSafeCommandExecutor_getSafeEnvironment(t *testing.T) {
	executor := NewSafeCommandExecutor()
	env := executor.getSafeEnvironment()
	foundPath := false
	for _, e := range env {
		if len(e) >= 5 && e[:5] == "PATH=" {
			foundPath = true
		}
	}
	if !foundPath {
		t.Error("PATH not found in safe environment")
	}
}

func TestIsCommandSafe(t *testing.T) {
	if !IsCommandSafe("ls") {
		t.Error("ls should be safe")
	}
	if IsCommandSafe("reboot") {
		t.Error("reboot should not be safe")
	}
}
