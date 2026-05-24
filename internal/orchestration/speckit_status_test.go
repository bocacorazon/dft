package orchestration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeSpecKitLaneReportsBlockingAnalyzeFromArtifacts(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	fixture.writeSpec(t)
	fixture.writePlan(t)
	fixture.writeTasks(t, "- [ ] T001 Concrete task\n")
	fixture.writeStepParsed(t, "analyze", map[string]any{
		"summary": map[string]any{
			"blocking_findings": 2,
			"high_findings":     2,
		},
	})

	summary, err := SummarizeSpecKitLane(fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("SummarizeSpecKitLane returned error: %v", err)
	}
	if summary.LatestSuccessfulStage != SpecKitStageTasks {
		t.Fatalf("latest successful stage = %q, want %q", summary.LatestSuccessfulStage, SpecKitStageTasks)
	}
	if summary.BlockingStage != SpecKitStageAnalyze {
		t.Fatalf("blocking stage = %q, want %q", summary.BlockingStage, SpecKitStageAnalyze)
	}
	if summary.AutomaticResumeSafe {
		t.Fatalf("automatic resume safe = true, want false for remediation-required analysis")
	}
}

func TestSummarizeSpecKitLaneMarksMissingPlanAsAutomatic(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	fixture.writeSpec(t)

	summary, err := SummarizeSpecKitLane(fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("SummarizeSpecKitLane returned error: %v", err)
	}
	if summary.BlockingStage != SpecKitStagePlan {
		t.Fatalf("blocking stage = %q, want %q", summary.BlockingStage, SpecKitStagePlan)
	}
	if !summary.AutomaticResumeSafe {
		t.Fatal("automatic resume safe = false, want true for start_stage recovery")
	}
}

func TestSummarizeSpecKitLaneClearsRecommendationWhenCompleted(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	fixture.writeSpec(t)
	fixture.writePlan(t)
	fixture.writeTasks(t, "- [X] T001 Complete task\n")
	fixture.writeStepParsed(t, "analyze", map[string]any{
		"summary": map[string]any{
			"blocking_findings": 0,
		},
	})
	fixture.writeStepParsed(t, "code-review", map[string]any{
		"summary": map[string]any{
			"critical_findings": 0,
		},
	})
	fixture.writeStepParsed(t, "issues-from-review", map[string]any{
		"issues": []any{},
	})
	if err := os.MkdirAll(filepath.Join(fixture.worktree.WorktreePath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	fixture.worktree.IncrementBranch = "main"
	fixture.writeStepParsed(t, "mergeback-finalize", map[string]any{
		"trees_equal":                      true,
		"local_branch_deleted":             true,
		"remote_branch_deleted_or_missing": true,
	})

	summary, err := SummarizeSpecKitLane(fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("SummarizeSpecKitLane returned error: %v", err)
	}
	if summary.ResumeRecommendation != "" {
		t.Fatalf("resume recommendation = %q, want empty for completed lane", summary.ResumeRecommendation)
	}
	if summary.AutomaticResumeSafe {
		t.Fatal("automatic resume safe = true, want false for completed lane")
	}
}
