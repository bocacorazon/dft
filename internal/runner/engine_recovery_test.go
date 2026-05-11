package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/bocacorazon/dft/internal/store"
)

type crashyAdapter struct {
	callCount int
	failOn2   bool
}

func (a *crashyAdapter) RunAgent(_ context.Context, workDir string, _ string, _ string) (string, error) {
	a.callCount++
	filename := filepath.Join(workDir, "artifact-"+strconv.Itoa(a.callCount)+".txt")
	if err := os.WriteFile(filename, []byte("ok"), 0o644); err != nil {
		return "", err
	}
	if a.failOn2 && a.callCount == 2 {
		return "", exec.ErrNotFound
	}
	return "ok", nil
}

func TestResumeSkipsCommittedStep(t *testing.T) {
	repoDir := t.TempDir()
	mustRun(t, repoDir, "git", "init", "-b", "main")
	mustRun(t, repoDir, "git", "config", "user.name", "DFT Test")
	mustRun(t, repoDir, "git", "config", "user.email", "dft@example.com")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("dft"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	mustRun(t, repoDir, "git", "add", "README.md")
	mustRun(t, repoDir, "git", "commit", "-m", "init")

	flowPath := filepath.Join(repoDir, "flow.yaml")
	flowYAML := []byte(`
steps:
  - id: step1
    type: agent
    agent: dft.test
    prompt: "one"
    capture: true
    export_as: one
  - id: step2
    type: agent
    agent: dft.test
    prompt: "{{ one }}"
    capture: true
`)
	if err := os.WriteFile(flowPath, flowYAML, 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	runStore := store.NewFilesystem(filepath.Join(repoDir, ".dft", "runs"))
	adapter := &crashyAdapter{failOn2: true}
	engine := NewEngine(runStore, adapter)

	runID, err := engine.Submit(context.Background(), flowPath)
	if err == nil {
		t.Fatalf("expected submit failure")
	}

	adapter.failOn2 = false
	resumedRunID, err := engine.Resume(context.Background(), runID)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if resumedRunID != runID {
		t.Fatalf("resumed run id mismatch: got %s want %s", resumedRunID, runID)
	}
	if adapter.callCount != 3 {
		t.Fatalf("expected 3 adapter calls (step1 + step2 fail + resume step2), got %d", adapter.callCount)
	}

	record, err := runStore.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if record.State != "succeeded" {
		t.Fatalf("expected succeeded after resume, got %s", record.State)
	}

	worktreePath := filepath.Join(repoDir, ".dft", "worktrees", runID)
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree cleanup, stat err=%v", err)
	}
}

func mustRun(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
}
