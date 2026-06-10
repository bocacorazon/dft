package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHarnessRun(t *testing.T) {
	// 1. Build dft executable
	dftPath := filepath.Join(os.TempDir(), "dft-test")
	cmd := exec.Command("go", "build", "-o", dftPath, "./../../../cmd/dft")
	// The path from internal/eval/harness to root is ../../../
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build dft: %v", err)
	}
	defer os.Remove(dftPath)

	// 2. Initialize harness with MockAgent
	mockAgent := &MockAgent{}
	h := New(dftPath, mockAgent, "stub")

	// 3. Execute Run
	ctx := context.Background()
	increment := "this is a mock increment"
	result, err := h.Run(ctx, increment)
	if err != nil {
		t.Fatalf("Harness.Run failed: %v", err)
	}

	// 4. Verification
	if result.RunID == "" {
		t.Error("Expected RunID to be generated, got empty")
	}
	if result.Increment != increment {
		t.Errorf("Expected increment %q, got %q", increment, result.Increment)
	}
	if result.WorkspaceDir == "" {
		t.Error("Expected WorkspaceDir, got empty")
	}
	if result.RemoteDir == "" {
		t.Error("Expected RemoteDir, got empty")
	}
	if result.DesignDir == "" {
		t.Error("Expected DesignDir, got empty")
	}
	if result.DesignError != "" {
		t.Errorf("Expected no DesignError, got %q", result.DesignError)
	}

	// Clean up created directories
	if result.WorkspaceDir != "" {
		defer os.RemoveAll(result.WorkspaceDir)
	}
	if result.RemoteDir != "" {
		defer os.RemoveAll(result.RemoteDir)
	}
}
