package github

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

func TestGitHubAdapterRunsFakeGhAndWritesRemoteAudits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh fixture is POSIX-specific")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "fake-gh")
	if err := os.WriteFile(binary, []byte(`#!/usr/bin/env sh
case "$1 $2" in
  "pr create") printf '42\n' ;;
  "pr list") printf '42\n' ;;
  "pr checks") printf 'checks passed\n' ;;
  "pr merge") printf 'merged\n' ;;
  *) printf 'unexpected %s\n' "$*" >&2; exit 2 ;;
esac
`), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	adapter := Adapter{RootDir: root, Binary: binary}

	pr, err := adapter.CreatePR(context.Background(), PRRequest{RunID: "run-123", StepID: "create-pr", Head: "increment/run-123", Base: "main", Title: "Run 123"})
	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}
	if pr.Number != 42 || !pr.RemoteOnly {
		t.Fatalf("PR record = %#v, want number 42 remote-only", pr)
	}
	if _, err := adapter.PRNumberForBranch(context.Background(), BranchPRRequest{RunID: "run-123", StepID: "number", Head: "increment/run-123"}); err != nil {
		t.Fatalf("PRNumberForBranch returned error: %v", err)
	}
	if _, err := adapter.WaitChecks(context.Background(), CheckRequest{RunID: "run-123", StepID: "checks", Number: 42}); err != nil {
		t.Fatalf("WaitChecks returned error: %v", err)
	}
	if _, err := adapter.MergePR(context.Background(), MergeRequest{RunID: "run-123", StepID: "merge", Number: 42}); err != nil {
		t.Fatalf("MergePR returned error: %v", err)
	}
	for _, stepID := range []string{"create-pr", "number", "checks", "merge"} {
		if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "remote", stepID+".json")); err != nil {
			t.Fatalf("missing audit for %s: %v", stepID, err)
		}
	}
}
