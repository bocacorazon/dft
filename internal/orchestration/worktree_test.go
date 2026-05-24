package orchestration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/ports"
)

func TestBeginIncrementCreatesBranchFromDefault(t *testing.T) {
	git := &recordingGit{defaultBranch: "main"}
	manager := WorktreeManager{Git: git, WorktreeRoot: ".dft/worktrees"}

	increment, err := manager.BeginIncrement(context.Background(), IncrementRequest{RunID: "run-123"})

	if err != nil {
		t.Fatalf("BeginIncrement returned error: %v", err)
	}
	if increment.DefaultBranch != "main" {
		t.Fatalf("default branch = %q, want main", increment.DefaultBranch)
	}
	if increment.Branch != "increment/run-123" {
		t.Fatalf("increment branch = %q, want increment/run-123", increment.Branch)
	}
	want := ports.CreateBranchRequest{Name: "increment/run-123", Base: "main"}
	if git.createdBranch != want {
		t.Fatalf("created branch = %#v, want %#v", git.createdBranch, want)
	}
}

func TestBeginSpecCreatesSpecBranchWorktreeAndSpecKitEnv(t *testing.T) {
	git := &recordingGit{defaultBranch: "main"}
	manager := WorktreeManager{Git: git, WorktreeRoot: ".dft/worktrees"}

	spec, err := manager.BeginSpec(context.Background(), SpecRequest{
		RunID:           "run-123",
		SpecID:          "001-intake",
		IncrementBranch: "increment/run-123",
	})

	if err != nil {
		t.Fatalf("BeginSpec returned error: %v", err)
	}
	if spec.Branch != "spec/run-123/001-intake" {
		t.Fatalf("spec branch = %q, want spec/run-123/001-intake", spec.Branch)
	}
	if spec.WorktreePath != filepath.Join(".dft", "worktrees", "run-123", "001-intake") {
		t.Fatalf("worktree path = %q", spec.WorktreePath)
	}
	if spec.IncrementBranch != "increment/run-123" {
		t.Fatalf("increment branch = %q, want increment/run-123", spec.IncrementBranch)
	}
	want := ports.CreateWorktreeRequest{
		Path:   filepath.Join(".dft", "worktrees", "run-123", "001-intake"),
		Branch: "spec/run-123/001-intake",
		Base:   "increment/run-123",
	}
	if git.createdWorktree != want {
		t.Fatalf("created worktree = %#v, want %#v", git.createdWorktree, want)
	}
	if got := spec.SpecKitEnv["GIT_BRANCH_NAME"]; got != "feature/001-intake" {
		t.Fatalf("GIT_BRANCH_NAME = %q, want explicit Speckit feature branch", got)
	}
}

func TestCompleteSpecMergesSpecBackToIncrement(t *testing.T) {
	git := &recordingGit{defaultBranch: "main"}
	manager := WorktreeManager{Git: git}

	err := manager.CompleteSpec(context.Background(), CompleteSpecRequest{
		SpecBranch:      "spec/run-123/001-intake",
		IncrementBranch: "increment/run-123",
	})

	if err != nil {
		t.Fatalf("CompleteSpec returned error: %v", err)
	}
	want := ports.MergeRequest{Source: "spec/run-123/001-intake", Target: "increment/run-123"}
	if git.merged != want {
		t.Fatalf("merge = %#v, want %#v", git.merged, want)
	}
}

type recordingGit struct {
	defaultBranch   string
	createdBranch   ports.CreateBranchRequest
	createdWorktree ports.CreateWorktreeRequest
	merged          ports.MergeRequest
}

func (g *recordingGit) DefaultBranch(context.Context) (string, error) {
	return g.defaultBranch, nil
}

func (g *recordingGit) CreateBranch(_ context.Context, request ports.CreateBranchRequest) error {
	g.createdBranch = request
	return nil
}

func (g *recordingGit) CreateWorktree(_ context.Context, request ports.CreateWorktreeRequest) error {
	g.createdWorktree = request
	return nil
}

func (g *recordingGit) Merge(_ context.Context, request ports.MergeRequest) error {
	g.merged = request
	return nil
}
