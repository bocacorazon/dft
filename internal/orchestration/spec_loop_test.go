package orchestration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/domain"
)

func TestPlanSpecsBuildsWBSAssignsLanesAndCreatesSpecWorktrees(t *testing.T) {
	root := t.TempDir()
	git := &recordingGit{defaultBranch: "main"}
	manager := WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")}
	orchestrator := SpecPlanner{
		Agent:        agentstub.Adapter{},
		Worktrees:    manager,
		ArtifactRoot: root,
	}

	result, err := orchestrator.PlanSpecs(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Build intake loop",
		RawDemand: "Build intake loop",
		AcceptanceCriteria: []string{
			"Demand package is generated.",
		},
	}, "increment/run-123")

	if err != nil {
		t.Fatalf("PlanSpecs returned error: %v", err)
	}
	if len(result.WBS.Specs) != 1 {
		t.Fatalf("spec count = %d, want 1", len(result.WBS.Specs))
	}
	if result.WBS.Specs[0].ID != "001-build-intake-loop" {
		t.Fatalf("spec id = %q", result.WBS.Specs[0].ID)
	}
	if len(result.LaneAssignments) != 1 || result.LaneAssignments[0].Lane != "spec" {
		t.Fatalf("lane assignments = %#v, want one spec lane", result.LaneAssignments)
	}
	if len(result.Worktrees) != 1 {
		t.Fatalf("worktree count = %d, want 1", len(result.Worktrees))
	}
	if got := result.Worktrees[0].SpecKitEnv["GIT_BRANCH_NAME"]; got != "spec/run-123/001-build-intake-loop" {
		t.Fatalf("GIT_BRANCH_NAME = %q", got)
	}

	for _, name := range []string{"wbs.json", "lane-assignments.json"} {
		path := filepath.Join(root, ".dft", "runs", "run-123", "design", name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected design artifact %s: %v", name, err)
		}
		assertValidJSONFile(t, path)
	}
}

func assertValidJSONFile(t *testing.T, path string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("%s is invalid JSON: %v\n%s", path, err, content)
	}
}
