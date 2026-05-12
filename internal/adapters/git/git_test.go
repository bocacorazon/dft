package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/ports"
	"github.com/bocacorazon/dft/internal/testutil"
)

func TestAdapterBranchWorktreeAndMerge(t *testing.T) {
	repo := testutil.TempGitRepo(t)
	writeFile(t, filepath.Join(repo, "README.md"), "root\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")

	adapter := Adapter{RepoDir: repo}
	defaultBranch, err := adapter.DefaultBranch(context.Background())
	if err != nil {
		t.Fatalf("DefaultBranch returned error: %v", err)
	}
	if defaultBranch != "main" {
		t.Fatalf("default branch = %q, want main", defaultBranch)
	}

	if err := adapter.CreateBranch(context.Background(), ports.CreateBranchRequest{
		Name: "increment/run-123",
		Base: "main",
	}); err != nil {
		t.Fatalf("CreateBranch returned error: %v", err)
	}

	worktreePath := filepath.Join(repo, ".dft", "worktrees", "run-123", "001-intake")
	if err := adapter.CreateWorktree(context.Background(), ports.CreateWorktreeRequest{
		Path:   worktreePath,
		Branch: "spec/run-123/001-intake",
		Base:   "increment/run-123",
	}); err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}

	writeFile(t, filepath.Join(worktreePath, "feature.txt"), "spec work\n")
	runGit(t, worktreePath, "add", "feature.txt")
	runGit(t, worktreePath, "commit", "-m", "spec work")

	if err := adapter.Merge(context.Background(), ports.MergeRequest{
		Source: "spec/run-123/001-intake",
		Target: "increment/run-123",
	}); err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}

	output := runGitOutput(t, repo, "branch", "--show-current")
	if strings.TrimSpace(output) != "increment/run-123" {
		t.Fatalf("current branch = %q, want increment/run-123", output)
	}
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); err != nil {
		t.Fatalf("merged file missing from increment branch: %v", err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	_ = runGitOutput(t, dir, args...)
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
