package orchestration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

type specKitArtifactFixture struct {
	root       string
	runID      string
	spec       domain.SpecRef
	worktree   SpecWorktree
	featureDir string
}

func newSpecKitArtifactFixture(t *testing.T) specKitArtifactFixture {
	t.Helper()
	root := t.TempDir()
	spec := domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
	}
	worktree := SpecWorktree{
		RunID:        "resume-run",
		SpecID:       spec.ID,
		WorktreePath: filepath.Join(root, ".dft", "worktrees", "resume-run", spec.ID),
	}
	featureDir := filepath.Join(worktree.WorktreePath, "specs", spec.ID)
	for path, content := range map[string]string{
		filepath.Join(root, ".specify", "templates", "spec-template.md"):  "# Spec Template\nTODO\n",
		filepath.Join(root, ".specify", "templates", "plan-template.md"):  "# Plan Template\nTODO\n",
		filepath.Join(root, ".specify", "templates", "tasks-template.md"): "# Tasks Template\n- [ ] TODO\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir template dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write template %s: %v", path, err)
		}
	}
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("mkdir feature dir: %v", err)
	}
	return specKitArtifactFixture{
		root:       root,
		runID:      worktree.RunID,
		spec:       spec,
		worktree:   worktree,
		featureDir: featureDir,
	}
}

func (f specKitArtifactFixture) writeSpec(t *testing.T) {
	t.Helper()
	f.writeFile(t, filepath.Join(f.featureDir, "spec.md"), "# Feature Spec\nConcrete requirements.\n")
}

func (f specKitArtifactFixture) writePlan(t *testing.T) {
	t.Helper()
	f.writeFile(t, filepath.Join(f.featureDir, "plan.md"), "# Implementation Plan\nConcrete plan.\n")
	f.writeFile(t, filepath.Join(f.featureDir, "research.md"), "# research.md\n- Decision: use concrete paths.\n")
}

func (f specKitArtifactFixture) writeTasks(t *testing.T, content string) {
	t.Helper()
	f.writeFile(t, filepath.Join(f.featureDir, "tasks.md"), content)
}

func (f specKitArtifactFixture) writeStepParsed(t *testing.T, stepID string, payload map[string]any) {
	t.Helper()
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s parsed payload: %v", stepID, err)
	}
	f.writeFile(t, filepath.Join(f.root, ".dft", "runs", f.runID, "steps", stepID, "parsed.json"), string(content)+"\n")
}

func (f specKitArtifactFixture) writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDecideSpecKitLaneResumeReturnsFirstIncompleteStageFromArtifacts(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	fixture.writeSpec(t)

	definition := BuildBaseSpecKitFlow(fixture.spec)
	decision, err := DecideSpecKitLaneResume(definition, fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("DecideSpecKitLaneResume returned error: %v", err)
	}
	if decision.Stage != SpecKitStagePlan {
		t.Fatalf("stage = %q, want %q", decision.Stage, SpecKitStagePlan)
	}
	if decision.ResumeStepID != "plan" {
		t.Fatalf("resume step = %q, want plan", decision.ResumeStepID)
	}
}

func TestDecideSpecKitLaneResumeUsesTasksRemediationForBlockingAnalyzeFindings(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	fixture.writeSpec(t)
	fixture.writePlan(t)
	fixture.writeTasks(t, "- [ ] T001 Concrete task\n")
	fixture.writeStepParsed(t, "analyze", map[string]any{
		"stdout": "blocking analyze output",
		"summary": map[string]any{
			"critical_findings": 0,
			"high_findings":     1,
			"blocking_findings": 1,
		},
	})

	definition := BuildBaseSpecKitFlow(fixture.spec)
	decision, err := DecideSpecKitLaneResume(definition, fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("DecideSpecKitLaneResume returned error: %v", err)
	}
	if decision.Stage != SpecKitStageAnalyze {
		t.Fatalf("stage = %q, want %q", decision.Stage, SpecKitStageAnalyze)
	}
	if decision.ResumeStepID != "tasks-remediation" {
		t.Fatalf("resume step = %q, want tasks-remediation", decision.ResumeStepID)
	}
	if decision.ResumeRecommendation != "remediate_and_resume" {
		t.Fatalf("resume recommendation = %q, want remediate_and_resume", decision.ResumeRecommendation)
	}
}

func TestDecideSpecKitLaneResumeReportsCompletedLaneFromArtifacts(t *testing.T) {
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

	definition := BuildBaseSpecKitFlow(fixture.spec)
	decision, err := DecideSpecKitLaneResume(definition, fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("DecideSpecKitLaneResume returned error: %v", err)
	}
	if !decision.Completed {
		t.Fatalf("decision = %#v, want completed lane", decision)
	}
}

func TestDecideSpecKitLaneResumeSkipsBackToMergebackFinalizeAfterSuccessfulRebase(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	fixture.worktree.IncrementBranch = "main"
	if err := os.MkdirAll(filepath.Join(fixture.worktree.WorktreePath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
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
	fixture.writeStepParsed(t, "mergeback-attempt", map[string]any{
		"status": "rebased",
		"source": "feature/001-real-submit",
		"target": "main",
	})

	definition := BuildBaseSpecKitFlow(fixture.spec)
	decision, err := DecideSpecKitLaneResume(definition, fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("DecideSpecKitLaneResume returned error: %v", err)
	}
	if decision.Stage != SpecKitStageMergeback {
		t.Fatalf("stage = %q, want %q", decision.Stage, SpecKitStageMergeback)
	}
	if decision.ResumeStepID != "mergeback-finalize" {
		t.Fatalf("resume step = %q, want mergeback-finalize", decision.ResumeStepID)
	}
}
