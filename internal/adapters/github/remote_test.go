package github

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDryRunPRCreateWritesRemoteOnlyAudit(t *testing.T) {
	root := t.TempDir()
	adapter := Adapter{RootDir: root, DryRun: true}

	record, err := adapter.CreatePR(context.Background(), PRRequest{
		RunID:  "run-123",
		StepID: "create-pr",
		Head:   "increment/run-123",
		Base:   "main",
		Title:  "Dogfood increment",
	})

	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}
	if !record.RemoteOnly {
		t.Fatalf("record remote_only = false, want true")
	}
	path := filepath.Join(root, ".dft", "runs", "run-123", "remote", "create-pr.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	var decoded PRRecord
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("audit invalid JSON: %v\n%s", err, content)
	}
}
