package execution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
	"github.com/bocacorazon/dft/internal/orchestration"
	"github.com/bocacorazon/dft/internal/ports"
)

// WorktreeExecutor runs a spec through the full Spec Kit lane
// (specify → plan → tasks → analyze → implement → code-review → mergeback) in an
// isolated per-spec git worktree. Per-spec code review and mergeback live in the
// provisioned lane YAML, so dft owns the quality of what it builds.
type WorktreeExecutor struct {
	Agent        ports.AgentAdapter
	Dispatcher   ports.CommandDispatcher
	Verifier     ports.Verifier
	Git          ports.GitPort
	ArtifactRoot string
	WorktreeRoot string
}

// Lane is the lane name this executor handles.
const WorktreeLane = "spec"

// Execute provisions a per-spec worktree, runs the Spec Kit lane inside it, and
// records the execution result.
func (w WorktreeExecutor) Execute(ctx context.Context, runID string, specID string, request domain.ExecutionRequest) (domain.ExecutionResult, error) {
	result := domain.ExecutionResult{
		RunID:       runID,
		SpecID:      specID,
		FeatureSlug: request.FeatureSlug,
		Status:      "failed",
	}
	if w.Agent == nil {
		err := fmt.Errorf("worktree executor requires an agent adapter")
		result.Error = err.Error()
		return result, err
	}
	if err := request.Validate(); err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.BaseBranch = request.BaseBranch
	result.IncrementBranch = request.IncrementBranch

	spec := specRefFromRequest(specID, request)
	worktree, err := w.provisionWorktree(ctx, runID, spec, request)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	definition, err := orchestration.LoadSpecKitLane(w.root(), spec, worktree)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("load spec kit lane: %w", err)
	}

	runner := w.runner(runID, worktree)
	flowResult, err := runner.Execute(ctx, definition)
	result.CompletedAt = time.Now().UTC()
	if err != nil {
		result.Error = err.Error()
		_ = writeJSON(filepath.Join(w.root(), ".dft", "runs", runID, "execution-result.json"), result)
		return result, fmt.Errorf("spec kit lane execution failed: %w", err)
	}
	if failed := firstFailedStep(flowResult); failed != "" {
		result.Error = fmt.Sprintf("step %q failed", failed)
		_ = writeJSON(filepath.Join(w.root(), ".dft", "runs", runID, "execution-result.json"), result)
		return result, fmt.Errorf("spec kit lane step %q failed", failed)
	}

	result.Status = "completed"
	if err := writeJSON(filepath.Join(w.root(), ".dft", "runs", runID, "execution-result.json"), result); err != nil {
		return result, err
	}
	return result, nil
}

// Status derives completeness for a spec lane from artifact truth rather than the
// journal, so an interrupted run resumes at the first unfinished stage.
func (w WorktreeExecutor) Status(_ context.Context, runID string, specID string, request domain.ExecutionRequest) (domain.ExecutionStatus, error) {
	status := domain.ExecutionStatus{RunID: runID, SpecID: specID}
	spec := specRefFromRequest(specID, request)
	worktree := w.buildWorktree(runID, spec, request)
	definition, err := orchestration.LoadSpecKitLane(w.root(), spec, worktree)
	if err != nil {
		status.Detail = "spec kit lane unavailable"
		return status, nil
	}
	decision, err := orchestration.DecideSpecKitLaneResume(definition, w.root(), runID, spec, worktree)
	if err != nil {
		status.Detail = "no lane artifacts recorded"
		return status, nil
	}
	status.Completed = decision.Completed
	status.ResumePoint = decision.ResumeStepID
	status.Detail = decision.ResumeRecommendation
	return status, nil
}

func (w WorktreeExecutor) provisionWorktree(ctx context.Context, runID string, spec domain.SpecRef, request domain.ExecutionRequest) (orchestration.SpecWorktree, error) {
	if w.Git == nil || strings.TrimSpace(request.IncrementBranch) == "" {
		return w.buildWorktree(runID, spec, request), nil
	}
	manager := orchestration.WorktreeManager{Git: w.Git, WorktreeRoot: w.worktreeRoot()}
	worktree, err := manager.BeginSpec(ctx, orchestration.SpecRequest{
		RunID:           runID,
		SpecID:          spec.ID,
		IncrementBranch: request.IncrementBranch,
	})
	if err != nil {
		return orchestration.SpecWorktree{}, fmt.Errorf("provision spec worktree: %w", err)
	}
	return worktree, nil
}

func (w WorktreeExecutor) buildWorktree(runID string, spec domain.SpecRef, request domain.ExecutionRequest) orchestration.SpecWorktree {
	return orchestration.SpecWorktree{
		RunID:           runID,
		SpecID:          spec.ID,
		Branch:          fmt.Sprintf("spec/%s/%s", runID, spec.ID),
		IncrementBranch: request.IncrementBranch,
		WorktreePath:    w.resolveWorktreePath(runID, spec.ID),
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": orchestration.SpecKitFeatureBranchName(spec.ID),
		},
	}
}

func (w WorktreeExecutor) runner(runID string, worktree orchestration.SpecWorktree) flow.Runner {
	return flow.Runner{
		Agent:            w.Agent,
		Dispatcher:       w.dispatcher(),
		ArtifactRoot:     w.root(),
		RunID:            runID,
		Verifier:         w.verifier(),
		CommitLocalSteps: worktreeHasGit(worktree.WorktreePath),
		AutoApproveGates: true,
	}
}

func (w WorktreeExecutor) dispatcher() ports.CommandDispatcher {
	if w.Dispatcher != nil {
		return w.Dispatcher
	}
	if dispatcher, ok := w.Agent.(ports.CommandDispatcher); ok {
		return dispatcher
	}
	if w.Agent == nil {
		return nil
	}
	return agentCommandDispatcher{agent: w.Agent}
}

func (w WorktreeExecutor) verifier() ports.Verifier {
	if w.Verifier != nil {
		return w.Verifier
	}
	return verify.Checker{RootDir: w.root()}
}

func (w WorktreeExecutor) resolveWorktreePath(runID string, specID string) string {
	root := w.worktreeRoot()
	return filepath.Join(root, runID, specID)
}

func (w WorktreeExecutor) worktreeRoot() string {
	if strings.TrimSpace(w.WorktreeRoot) != "" {
		return w.WorktreeRoot
	}
	return filepath.Join(".dft", "worktrees")
}

func (w WorktreeExecutor) root() string {
	if strings.TrimSpace(w.ArtifactRoot) == "" {
		return "."
	}
	return w.ArtifactRoot
}

func specRefFromRequest(specID string, request domain.ExecutionRequest) domain.SpecRef {
	return domain.SpecRef{
		ID:          specID,
		Description: request.Prompt,
		PromptPath:  request.PromptPath,
		FeatureSlug: request.FeatureSlug,
	}
}

func firstFailedStep(result flow.Result) string {
	for _, step := range result.Steps {
		if step.Status == flow.StepFailed {
			return step.ID
		}
	}
	return ""
}

func worktreeHasGit(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

var _ Executor = WorktreeExecutor{}
