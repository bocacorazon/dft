package orchestration

import (
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestBuildSpecKitLanePassesBranchAndFeatureDirectoryEnvironment(t *testing.T) {
	definition := BuildSpecKitLane(domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
	}, SpecWorktree{
		Branch:       "spec/run-123/001-real-submit",
		WorktreePath: ".dft/worktrees/run-123/001-real-submit",
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": "spec/run-123/001-real-submit",
		},
	})

	if len(definition.Steps) != 4 {
		t.Fatalf("step count = %d, want 4", len(definition.Steps))
	}
	wantAgents := []string{
		"speckit.specify.agent.md",
		"speckit.plan.agent.md",
		"speckit.tasks.agent.md",
		"speckit.implement.agent.md",
	}
	for i, want := range wantAgents {
		if definition.Steps[i].AgentName != want {
			t.Fatalf("step %d agent = %q, want %q", i, definition.Steps[i].AgentName, want)
		}
		if got := definition.Steps[i].Env["GIT_BRANCH_NAME"]; got != "spec/run-123/001-real-submit" {
			t.Fatalf("step %d GIT_BRANCH_NAME = %q", i, got)
		}
		if got := definition.Steps[i].Env["SPECIFY_FEATURE_DIRECTORY"]; got != "specs/001-real-submit" {
			t.Fatalf("step %d SPECIFY_FEATURE_DIRECTORY = %q", i, got)
		}
		if got := definition.Steps[i].Cwd; got != ".dft/worktrees/run-123/001-real-submit" {
			t.Fatalf("step %d Cwd = %q", i, got)
		}
	}
}
