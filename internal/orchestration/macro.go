package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	dfteval "github.com/bocacorazon/dft/internal/eval"
	"github.com/bocacorazon/dft/internal/flow"
	"github.com/bocacorazon/dft/internal/ports"
	"github.com/bocacorazon/dft/internal/review"
)

// MacroOrchestrator implements the top-level v1 dft lifecycle as engine code.
type MacroOrchestrator struct {
	Agent            ports.AgentAdapter
	Worktrees        WorktreeManager
	Verifier         ports.Verifier
	ArtifactRoot     string
	Review           domain.ReviewDecision
	CommitLocalSteps bool
	HoldIncrement    bool
	MaxEvalRetries   int
}

// MacroResult summarizes a completed macro-orchestration run.
type MacroResult struct {
	Increment        Increment                 `json:"increment"`
	SpecPlan         SpecPlanResult            `json:"spec_plan"`
	StepResults      []flow.StepResult         `json:"step_results"`
	EvalPlan         domain.EvalPlan           `json:"eval_plan"`
	EvalReady        domain.EvalReady          `json:"eval_ready"`
	EvalResult       domain.EvalResult         `json:"eval_result"`
	Evaluation       domain.VerificationResult `json:"evaluation"`
	WBSAmendment     *domain.WBSAmendment      `json:"wbs_amendment,omitempty"`
	Review           domain.ReviewDecision     `json:"review"`
	FinalMergeTarget string                    `json:"final_merge_target"`
	IncrementHeld    bool                      `json:"increment_held,omitempty"`
}

// Execute drives one demand package through increment setup, design,
// orchestration, eval, and final reviewed merge.
func (m MacroOrchestrator) Execute(ctx context.Context, demandPackage domain.DemandPackage) (MacroResult, error) {
	if err := demandPackage.Validate(); err != nil {
		return MacroResult{}, fmt.Errorf("validate demand package: %w", err)
	}
	if m.Agent == nil {
		return MacroResult{}, fmt.Errorf("agent adapter is required")
	}
	if m.Verifier == nil {
		return MacroResult{}, fmt.Errorf("verifier is required")
	}

	increment, err := m.Worktrees.BeginIncrement(ctx, IncrementRequest{RunID: demandPackage.ID})
	if err != nil {
		return MacroResult{}, fmt.Errorf("begin increment: %w", err)
	}

	specPlan, err := (SpecPlanner{
		Agent:        m.Agent,
		Worktrees:    m.Worktrees,
		ArtifactRoot: m.ArtifactRoot,
	}).PlanSpecs(ctx, demandPackage, increment.Branch)
	if err != nil {
		return MacroResult{}, fmt.Errorf("plan specs: %w", err)
	}

	runner := flow.Runner{
		Agent:            m.Agent,
		Dispatcher:       commandDispatcher(m.Agent),
		ArtifactRoot:     m.ArtifactRoot,
		RunID:            demandPackage.ID,
		Verifier:         verify.Checker{RootDir: m.ArtifactRoot},
		CommitLocalSteps: m.CommitLocalSteps,
		AutoApproveGates: true,
	}
	stepResults, err := m.executeSpecs(ctx, runner, demandPackage.ID, increment.Branch, specPlan.WBS.Specs, specPlan.Worktrees)
	if err != nil {
		return MacroResult{}, err
	}

	evalRun, evaluation, err := m.evaluateIncrement(ctx, demandPackage, specPlan)
	if err != nil {
		return MacroResult{}, fmt.Errorf("evaluate increment: %w", err)
	}
	result := MacroResult{
		Increment:        increment,
		SpecPlan:         specPlan,
		StepResults:      stepResults,
		EvalPlan:         evalRun.Plan,
		EvalReady:        evalRun.Ready,
		EvalResult:       evalRun.Result,
		Evaluation:       evaluation,
		FinalMergeTarget: increment.DefaultBranch,
		IncrementHeld:    m.HoldIncrement,
	}
	for attempt := 0; evaluation.Status != domain.VerdictPass && attempt < m.MaxEvalRetries; attempt++ {
		amendment, err := (review.FixPlanner{
			Agent:        m.Agent,
			ArtifactRoot: m.ArtifactRoot,
			RunID:        demandPackage.ID,
		}).Plan(ctx, demandPackage, evaluation)
		if err != nil {
			return MacroResult{}, fmt.Errorf("plan failed-eval remediation: %w", err)
		}
		result.WBSAmendment = &amendment
		remediationResults, err := m.executeSpecs(ctx, runner, demandPackage.ID, increment.Branch, amendment.RemediationSpecs, nil)
		stepResults = append(stepResults, remediationResults...)
		result.StepResults = stepResults
		if err != nil {
			return MacroResult{}, err
		}
		evalRun, evaluation, err = m.evaluateIncrement(ctx, demandPackage, specPlan)
		if err != nil {
			return MacroResult{}, fmt.Errorf("evaluate remediation attempt %d: %w", attempt+1, err)
		}
		result.EvalPlan = evalRun.Plan
		result.EvalReady = evalRun.Ready
		result.EvalResult = evalRun.Result
		result.Evaluation = evaluation
	}
	if evaluation.Status != domain.VerdictPass {
		if result.WBSAmendment == nil {
			amendment, err := (review.FixPlanner{
				Agent:        m.Agent,
				ArtifactRoot: m.ArtifactRoot,
				RunID:        demandPackage.ID,
			}).Plan(ctx, demandPackage, evaluation)
			if err != nil {
				return MacroResult{}, fmt.Errorf("plan failed-eval remediation: %w", err)
			}
			result.WBSAmendment = &amendment
		}
		if err := writeMacroResult(m.ArtifactRoot, demandPackage.ID, result); err != nil {
			return MacroResult{}, err
		}
		return result, fmt.Errorf("evaluation failed; WBS amendment written for %d finding(s)", len(evaluation.Findings))
	}

	reviewDecision, err := m.reviewIncrement(ctx, demandPackage, increment.Branch)
	if err != nil {
		return MacroResult{}, err
	}
	result.Review = reviewDecision
	for attempt := 0; !reviewDecision.Approved && attempt < m.MaxEvalRetries; attempt++ {
		amendment, err := (review.FixPlanner{
			Agent:        m.Agent,
			ArtifactRoot: m.ArtifactRoot,
			RunID:        demandPackage.ID,
		}).Plan(ctx, demandPackage, verificationFromReview(reviewDecision))
		if err != nil {
			return MacroResult{}, fmt.Errorf("plan failed-review remediation: %w", err)
		}
		result.WBSAmendment = &amendment
		remediationResults, err := m.executeSpecs(ctx, runner, demandPackage.ID, increment.Branch, amendment.RemediationSpecs, nil)
		stepResults = append(stepResults, remediationResults...)
		result.StepResults = stepResults
		if err != nil {
			return MacroResult{}, err
		}
		evalRun, evaluation, err = m.evaluateIncrement(ctx, demandPackage, specPlan)
		if err != nil {
			return MacroResult{}, fmt.Errorf("evaluate review remediation attempt %d: %w", attempt+1, err)
		}
		result.EvalPlan = evalRun.Plan
		result.EvalReady = evalRun.Ready
		result.EvalResult = evalRun.Result
		result.Evaluation = evaluation
		if evaluation.Status != domain.VerdictPass {
			break
		}

		reviewDecision, err = m.reviewIncrement(ctx, demandPackage, increment.Branch)
		if err != nil {
			return MacroResult{}, err
		}
		result.Review = reviewDecision
	}
	if !reviewDecision.Approved {
		if err := writeInboxReviewBlock(m.ArtifactRoot, demandPackage.ID, reviewDecision); err != nil {
			return MacroResult{}, err
		}
		if err := writeMacroResult(m.ArtifactRoot, demandPackage.ID, result); err != nil {
			return MacroResult{}, err
		}
		return result, fmt.Errorf("final review blocked increment with %d finding(s)", len(reviewDecision.Findings))
	}
	if !m.HoldIncrement {
		if err := m.Worktrees.CompleteIncrement(ctx, CompleteIncrementRequest{
			IncrementBranch: increment.Branch,
			DefaultBranch:   increment.DefaultBranch,
			Evaluation:      evaluation,
			Review:          reviewDecision,
		}); err != nil {
			return MacroResult{}, fmt.Errorf("complete increment: %w", err)
		}
	}

	if err := writeMacroResult(m.ArtifactRoot, demandPackage.ID, result); err != nil {
		return MacroResult{}, err
	}
	return result, nil
}

func (m MacroOrchestrator) evaluateIncrement(ctx context.Context, demandPackage domain.DemandPackage, specPlan SpecPlanResult) (dfteval.Result, domain.VerificationResult, error) {
	manifest := dfteval.ManifestFromSurfaceContract(demandPackage.ID, specPlan.EvalSurfaceContract)
	evalRun, err := (dfteval.Orchestrator{
		Agent:        m.Agent,
		Verifier:     m.Verifier,
		ArtifactRoot: m.ArtifactRoot,
		RunID:        demandPackage.ID,
	}).Run(ctx, dfteval.AuthorInput{
		DemandPackage:    demandPackage,
		WBS:              specPlan.WBS,
		SurfaceContract:  specPlan.EvalSurfaceContract,
		ArtifactManifest: manifest,
		StepCatalog:      dfteval.DefaultStepCatalog(),
	})
	if err != nil {
		return dfteval.Result{}, domain.VerificationResult{}, err
	}
	return evalRun, dfteval.ToVerificationResult(evalRun.Result), nil
}

func (m MacroOrchestrator) reviewIncrement(ctx context.Context, demandPackage domain.DemandPackage, incrementBranch string) (domain.ReviewDecision, error) {
	reviewDecision := m.Review
	if !reviewDecision.Approved && len(reviewDecision.Findings) == 0 {
		var err error
		reviewDecision, err = (review.FinalReviewer{
			Agent:        m.Agent,
			ArtifactRoot: m.ArtifactRoot,
			RunID:        demandPackage.ID,
		}).Review(ctx, demandPackage, incrementBranch)
		if err != nil {
			return domain.ReviewDecision{}, fmt.Errorf("final review: %w", err)
		}
	}
	return reviewDecision, nil
}

func verificationFromReview(decision domain.ReviewDecision) domain.VerificationResult {
	return domain.VerificationResult{
		Status:   domain.VerdictFail,
		Findings: decision.Findings,
	}
}

func (m MacroOrchestrator) executeSpecs(ctx context.Context, runner flow.Runner, runID string, incrementBranch string, specs []domain.SpecRef, worktrees []SpecWorktree) ([]flow.StepResult, error) {
	var stepResults []flow.StepResult
	for i, spec := range specs {
		worktree := SpecWorktree{
			Branch:       "spec/" + runID + "/" + spec.ID,
			SpecID:       spec.ID,
			WorktreePath: filepath.Join(".dft", "worktrees", runID, spec.ID),
			SpecKitEnv: map[string]string{
				"GIT_BRANCH_NAME": SpecKitFeatureBranchName(spec.ID),
			},
		}
		if i < len(worktrees) {
			worktree = worktrees[i]
		} else {
			var err error
			worktree, err = m.Worktrees.BeginSpec(ctx, SpecRequest{
				RunID:           runID,
				SpecID:          spec.ID,
				IncrementBranch: incrementBranch,
			})
			if err != nil {
				return stepResults, fmt.Errorf("begin spec %s: %w", spec.ID, err)
			}
		}
		definition, err := LoadSpecKitLane(m.ArtifactRoot, spec, worktree)
		if err != nil {
			return stepResults, fmt.Errorf("load spec workflow %s: %w", spec.ID, err)
		}
		specRunner := runner
		observer, err := newSpecKitLaneJournalObserver(m.ArtifactRoot, runID, spec.ID)
		if err != nil {
			return stepResults, fmt.Errorf("create lane journal observer for spec %s: %w", spec.ID, err)
		}
		specRunner.Observer = observer
		result, err := specRunner.Execute(ctx, definition)
		stepResults = append(stepResults, result.Steps...)
		if err != nil {
			return stepResults, fmt.Errorf("run spec %s: %w", spec.ID, err)
		}
		if !specLaneCompletedMergeback(result.StepOutputs) {
			if err := m.Worktrees.CompleteSpec(ctx, CompleteSpecRequest{
				SpecBranch:      worktree.Branch,
				IncrementBranch: incrementBranch,
			}); err != nil {
				return stepResults, fmt.Errorf("complete spec %s: %w", spec.ID, err)
			}
		}
	}
	return stepResults, nil
}

func specLaneCompletedMergeback(stepOutputs map[string]map[string]any) bool {
	output, ok := stepOutputs["mergeback-finalize"]
	if !ok {
		return false
	}
	return output["status"] == "merged" && output["trees_equal"] == true
}

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

func writeMacroResult(root string, runID string, result MacroResult) error {
	path := filepath.Join(root, ".dft", "runs", runID, "macro-result.json")
	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode macro result: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write macro result: %w", err)
	}
	return nil
}

func writeInboxReviewBlock(root string, runID string, decision domain.ReviewDecision) error {
	path := filepath.Join(root, ".dft", "inbox", "review-blocked-"+runID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create inbox directory: %w", err)
	}
	content, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		return fmt.Errorf("encode review block: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write review block: %w", err)
	}
	return nil
}
