package v2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	domainv2 "github.com/bocacorazon/dft/internal/domain/v2"
	"github.com/bocacorazon/dft/internal/flow"
	"github.com/bocacorazon/dft/internal/orchestration"
	"github.com/bocacorazon/dft/internal/ports"
)

// SpeckitExecutor runs a spec through the full Speckit lane:
// specify → plan → tasks → analyze → implement → code-review → mergeback.
// Per-spec review (code-review agent) is automated within the executor —
// it loops implement/review until no CRITICAL findings, then proceeds to mergeback.
type SpeckitExecutor struct {
	worktreeRoot string
}

// NewSpeckitExecutor creates a Speckit executor with the given worktree root directory.
func NewSpeckitExecutor(worktreeRoot string) *SpeckitExecutor {
	return &SpeckitExecutor{worktreeRoot: worktreeRoot}
}

func (s *SpeckitExecutor) Name() string { return "speckit" }

func (s *SpeckitExecutor) Execute(ctx context.Context, input ExecutorInput) (domainv2.SpecResult, error) {
	if input.Agent == nil {
		return domainv2.SpecResult{}, fmt.Errorf("speckit executor requires an agent adapter")
	}

	worktree := s.buildWorktree(input)
	definition, err := orchestration.LoadSpecKitLane(input.ArtifactRoot, input.Spec, worktree)
	if err != nil {
		return domainv2.SpecResult{}, fmt.Errorf("load speckit lane: %w", err)
	}

	dispatcher := commandDispatcher(input.Agent)
	runner := flow.Runner{
		Agent:            input.Agent,
		Dispatcher:       dispatcher,
		ArtifactRoot:     input.ArtifactRoot,
		RunID:            input.RunID,
		Verifier:         verify.Checker{RootDir: input.ArtifactRoot},
		CommitLocalSteps: worktreeHasGit(worktree.WorktreePath),
		AutoApproveGates: true,
	}

	result, err := runner.Execute(ctx, definition)
	if err != nil {
		return domainv2.SpecResult{
			SpecID: input.Spec.ID,
			Lane:   "speckit",
			Status: "failed",
			Findings: []domain.Finding{
				{Severity: "CRITICAL", Message: err.Error()},
			},
		}, fmt.Errorf("speckit lane execution failed: %w", err)
	}

	status := "completed"
	for _, step := range result.Steps {
		if step.Status == flow.StepFailed {
			status = "failed"
			break
		}
	}

	return domainv2.SpecResult{
		SpecID:  input.Spec.ID,
		Lane:    "speckit",
		Status:  status,
		Branch:  worktree.Branch,
	}, nil
}

func (s *SpeckitExecutor) buildWorktree(input ExecutorInput) orchestration.SpecWorktree {
	specBranch := fmt.Sprintf("spec/%s/%s", input.RunID, input.Spec.ID)
	return orchestration.SpecWorktree{
		RunID:           input.RunID,
		SpecID:          input.Spec.ID,
		Branch:          specBranch,
		IncrementBranch: input.IncrementBranch,
		WorktreePath:    s.resolveWorktreePath(input),
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": orchestration.SpecKitFeatureBranchName(input.Spec.ID),
		},
	}
}

func (s *SpeckitExecutor) resolveWorktreePath(input ExecutorInput) string {
	if input.WorktreePath != "" {
		return input.WorktreePath
	}
	if s.worktreeRoot != "" {
		return filepath.Join(s.worktreeRoot, input.RunID, input.Spec.ID)
	}
	return filepath.Join(".dft", "worktrees", input.RunID, input.Spec.ID)
}

// commandDispatcher wraps an AgentAdapter as a CommandDispatcher for the flow runner.
func commandDispatcher(agent ports.AgentAdapter) ports.CommandDispatcher {
	if dispatcher, ok := agent.(ports.CommandDispatcher); ok {
		return dispatcher
	}
	return agentCommandDispatcher{agent: agent}
}

type agentCommandDispatcher struct {
	agent ports.AgentAdapter
}

func (d agentCommandDispatcher) DispatchCommand(ctx context.Context, request ports.CommandRequest) (ports.CommandResponse, error) {
	response, err := d.agent.Invoke(ctx, ports.AgentRequest{
		AgentName:  request.Command + ".agent.md",
		Prompt:     request.Input,
		Demand:     request.Input,
		RunID:      request.RunID,
		Cwd:        request.Cwd,
		Env:        request.Env,
		AllowTools: request.AllowTools,
	})
	if err != nil {
		return ports.CommandResponse{}, err
	}
	return ports.CommandResponse{Stdout: response.Raw, ExitCode: 0}, nil
}

func worktreeHasGit(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// Ensure SpeckitExecutor implements Executor.
var _ Executor = (*SpeckitExecutor)(nil)