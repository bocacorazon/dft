package execution

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestRegistryForResolvesLaneAndDefault(t *testing.T) {
	def := recordingExecutor{name: "default"}
	lane := recordingExecutor{name: "spec"}
	registry := NewRegistry(&def)
	registry.Register("spec", &lane)

	got, ok := registry.For("spec")
	if !ok || got.(*recordingExecutor).name != "spec" {
		t.Fatalf("For(spec) = %v, %v; want spec executor", got, ok)
	}
	got, ok = registry.For("")
	if !ok || got.(*recordingExecutor).name != "default" {
		t.Fatalf("For(\"\") = %v, %v; want default executor", got, ok)
	}
	got, ok = registry.For("unregistered")
	if !ok || got.(*recordingExecutor).name != "default" {
		t.Fatalf("For(unregistered) = %v, %v; want default fallback", got, ok)
	}
}

func TestServiceStatusReflectsRecordedResult(t *testing.T) {
	root := t.TempDir()
	service := Service{ArtifactRoot: root}

	status, err := service.Status(context.Background(), "run-x", "001", domain.ExecutionRequest{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.Completed {
		t.Fatalf("status before any run should not be completed")
	}

	writeResult(t, root, "run-x", domain.ExecutionResult{RunID: "run-x", Status: "completed"})
	status, err = service.Status(context.Background(), "run-x", "001", domain.ExecutionRequest{})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !status.Completed {
		t.Fatalf("status after completed run should be completed")
	}
}

func TestOrchestratorSelectsLaneExecutorAndSkipsCompletedSpecs(t *testing.T) {
	root := t.TempDir()
	defaultExec := &recordingExecutor{name: "default"}
	laneExec := &recordingExecutor{name: "spec", completedSpecs: map[string]bool{"002-done": true}}
	registry := NewRegistry(defaultExec)
	registry.Register("spec", laneExec)

	wbs := domain.WBS{
		IncrementPackageID: "run-1",
		BaseBranch:         "main",
		IncrementBranch:    "increment/run-1",
		Specs: []domain.SpecRef{
			{ID: "001-default", Description: "Base", AcceptanceCriteria: []string{"x"}},
			{ID: "002-done", Description: "Already done", Lane: "spec", AcceptanceCriteria: []string{"y"}},
		},
	}
	content, err := json.Marshal(wbs)
	if err != nil {
		t.Fatalf("marshal wbs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "wbs.json"), content, 0o644); err != nil {
		t.Fatalf("write wbs: %v", err)
	}

	orchestrator := Orchestrator{Registry: registry, ArtifactRoot: root}
	result, err := orchestrator.Execute(context.Background(), "run-1", domain.OrchestrationRequest{
		WBSPath:         "wbs.json",
		BaseBranch:      "main",
		ModelConfigPath: "models.json",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if len(defaultExec.executed) != 1 || defaultExec.executed[0] != "001-default" {
		t.Fatalf("default executor ran %v, want [001-default]", defaultExec.executed)
	}
	if len(laneExec.executed) != 0 {
		t.Fatalf("lane executor ran %v, want none (spec already complete)", laneExec.executed)
	}
}

func TestWorktreeExecutorDerivesPerSpecWorktree(t *testing.T) {
	exec := WorktreeExecutor{ArtifactRoot: "/repo", WorktreeRoot: filepath.Join("/repo", ".dft", "worktrees")}
	spec := specRefFromRequest("001-auth", domain.ExecutionRequest{Prompt: "Build auth", IncrementBranch: "increment/run-1"})
	worktree := exec.buildWorktree("run-1", spec, domain.ExecutionRequest{IncrementBranch: "increment/run-1"})

	if got, want := worktree.Branch, "spec/run-1/001-auth"; got != want {
		t.Fatalf("branch = %q, want %q", got, want)
	}
	if got, want := worktree.WorktreePath, filepath.Join("/repo", ".dft", "worktrees", "run-1", "001-auth"); got != want {
		t.Fatalf("worktree path = %q, want %q", got, want)
	}
	if got := worktree.SpecKitEnv["GIT_BRANCH_NAME"]; got != "feature/001-auth" {
		t.Fatalf("GIT_BRANCH_NAME = %q, want feature/001-auth", got)
	}
}

func TestWorktreeExecutorStatusIsGracefulWithoutArtifacts(t *testing.T) {
	exec := WorktreeExecutor{ArtifactRoot: t.TempDir()}
	status, err := exec.Status(context.Background(), "run-1", "001-auth", domain.ExecutionRequest{IncrementBranch: "increment/run-1"})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.Completed {
		t.Fatalf("status without artifacts should not be completed")
	}
}

func TestWorktreeExecutorResumeSkipsCompletedLaneWithoutAgent(t *testing.T) {
	root := t.TempDir()
	const runID = "resume-run"
	const specID = "001-real-submit"
	writeCompletedLaneArtifacts(t, root, runID, specID)

	agent := &failingAgent{t: t}
	exec := WorktreeExecutor{
		Agent:        agent,
		ArtifactRoot: root,
		WorktreeRoot: filepath.Join(root, ".dft", "worktrees"),
	}
	request := domain.ExecutionRequest{
		Prompt:          "Enable real submit",
		Workflow:        "spec-kit",
		FeatureSlug:     "real-submit",
		ModelConfigPath: "models.json",
	}

	result, err := exec.Execute(context.Background(), runID, specID, request)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if agent.invoked {
		t.Fatalf("agent should not be invoked when the lane is already complete")
	}
}

// writeCompletedLaneArtifacts lays down the artifact set that DecideSpecKitLaneResume
// recognizes as a fully completed Spec Kit lane (mirrors the orchestration fixture).
func writeCompletedLaneArtifacts(t *testing.T, root, runID, specID string) {
	t.Helper()
	worktreePath := filepath.Join(root, ".dft", "worktrees", runID, specID)
	featureDir := filepath.Join(worktreePath, "specs", specID)
	writeFileForTest(t, filepath.Join(root, ".specify", "templates", "spec-template.md"), "# Spec Template\nTODO\n")
	writeFileForTest(t, filepath.Join(root, ".specify", "templates", "plan-template.md"), "# Plan Template\nTODO\n")
	writeFileForTest(t, filepath.Join(root, ".specify", "templates", "tasks-template.md"), "# Tasks Template\n- [ ] TODO\n")
	writeFileForTest(t, filepath.Join(featureDir, "spec.md"), "# Feature Spec\nConcrete requirements.\n")
	writeFileForTest(t, filepath.Join(featureDir, "plan.md"), "# Implementation Plan\nConcrete plan.\n")
	writeFileForTest(t, filepath.Join(featureDir, "research.md"), "# research.md\n- Decision: use concrete paths.\n")
	writeFileForTest(t, filepath.Join(featureDir, "tasks.md"), "- [X] T001 Complete task\n")
	writeStepParsedForTest(t, root, runID, "analyze", map[string]any{"summary": map[string]any{"blocking_findings": 0}})
	writeStepParsedForTest(t, root, runID, "code-review", map[string]any{"summary": map[string]any{"critical_findings": 0}})
	writeStepParsedForTest(t, root, runID, "issues-from-review", map[string]any{"issues": []any{}})
}

func writeStepParsedForTest(t *testing.T, root, runID, stepID string, payload map[string]any) {
	t.Helper()
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s parsed payload: %v", stepID, err)
	}
	writeFileForTest(t, filepath.Join(root, ".dft", "runs", runID, "steps", stepID, "parsed.json"), string(content)+"\n")
}

func writeFileForTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

type failingAgent struct {
	t       *testing.T
	invoked bool
}

func (f *failingAgent) Invoke(context.Context, ports.AgentRequest) (ports.AgentResponse, error) {
	f.invoked = true
	f.t.Errorf("agent Invoke called unexpectedly for a completed lane")
	return ports.AgentResponse{}, nil
}

type recordingExecutor struct {
	name           string
	completedSpecs map[string]bool
	executed       []string
}

func (e *recordingExecutor) Execute(_ context.Context, runID string, specID string, request domain.ExecutionRequest) (domain.ExecutionResult, error) {
	e.executed = append(e.executed, specID)
	return domain.ExecutionResult{
		RunID:           runID,
		SpecID:          specID,
		FeatureSlug:     request.FeatureSlug,
		BaseBranch:      request.BaseBranch,
		IncrementBranch: request.IncrementBranch,
		Status:          "completed",
	}, nil
}

func (e *recordingExecutor) Status(_ context.Context, runID string, specID string, _ domain.ExecutionRequest) (domain.ExecutionStatus, error) {
	return domain.ExecutionStatus{RunID: runID, SpecID: specID, Completed: e.completedSpecs[specID]}, nil
}

func writeResult(t *testing.T, root string, runID string, result domain.ExecutionResult) {
	t.Helper()
	path := filepath.Join(root, ".dft", "runs", runID, "execution-result.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write result: %v", err)
	}
}
