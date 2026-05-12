package orchestration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestMacroOrchestratorRunsFullLocalIncrementLifecycle(t *testing.T) {
	root := t.TempDir()
	git := &macroRecordingGit{defaultBranch: "main"}
	orchestrator := MacroOrchestrator{
		Agent:        agentstub.Adapter{},
		Worktrees:    WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")},
		Verifier:     verify.Checker{RootDir: root},
		ArtifactRoot: root,
		Review:       domain.ReviewDecision{Approved: true},
	}

	result, err := orchestrator.Execute(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Macro orchestrator",
		RawDemand: "Macro orchestrator",
		AcceptanceCriteria: []string{
			"Full local increment lifecycle completes.",
		},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Increment.Branch != "increment/run-123" {
		t.Fatalf("increment branch = %q, want increment/run-123", result.Increment.Branch)
	}
	if result.Evaluation.Status != domain.VerdictPass {
		t.Fatalf("evaluation = %#v, want pass", result.Evaluation)
	}
	if len(git.merges) != 2 {
		t.Fatalf("merge count = %d, want spec merge and final merge", len(git.merges))
	}
	if got := git.merges[1]; got != (ports.MergeRequest{Source: "increment/run-123", Target: "main"}) {
		t.Fatalf("final merge = %#v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "macro-result.json")); err != nil {
		t.Fatalf("macro result artifact missing: %v", err)
	}
}

type macroRecordingGit struct {
	defaultBranch string
	merges        []ports.MergeRequest
}

func (g *macroRecordingGit) DefaultBranch(context.Context) (string, error) {
	return g.defaultBranch, nil
}

func (g *macroRecordingGit) CreateBranch(context.Context, ports.CreateBranchRequest) error {
	return nil
}

func (g *macroRecordingGit) CreateWorktree(context.Context, ports.CreateWorktreeRequest) error {
	return nil
}

func (g *macroRecordingGit) Merge(_ context.Context, request ports.MergeRequest) error {
	g.merges = append(g.merges, request)
	return nil
}
