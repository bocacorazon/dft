package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
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
	EvalPlan         domain.EvaluationPlan     `json:"eval_plan"`
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
		ArtifactRoot:     m.ArtifactRoot,
		RunID:            demandPackage.ID,
		Verifier:         m.Verifier,
		CommitLocalSteps: m.CommitLocalSteps,
	}
	stepResults, err := m.executeSpecs(ctx, runner, demandPackage.ID, increment.Branch, specPlan.WBS.Specs, specPlan.Worktrees)
	if err != nil {
		return MacroResult{}, err
	}

	evalPlan, err := (review.EvalPlanAuthor{
		Agent:        m.Agent,
		ArtifactRoot: m.ArtifactRoot,
		RunID:        demandPackage.ID,
	}).Author(ctx, demandPackage)
	if err != nil {
		return MacroResult{}, fmt.Errorf("author eval plan: %w", err)
	}

	evaluator := review.Evaluator{
		Verifier:     m.Verifier,
		ArtifactRoot: m.ArtifactRoot,
		RunID:        demandPackage.ID,
	}
	evaluation, err := evaluator.EvaluatePlan(ctx, evalPlan)
	if err != nil {
		return MacroResult{}, fmt.Errorf("evaluate increment: %w", err)
	}
	result := MacroResult{
		Increment:        increment,
		SpecPlan:         specPlan,
		StepResults:      stepResults,
		EvalPlan:         evalPlan,
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
		evaluation, err = evaluator.EvaluatePlan(ctx, evalPlan)
		if err != nil {
			return MacroResult{}, fmt.Errorf("evaluate remediation attempt %d: %w", attempt+1, err)
		}
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

	reviewDecision := m.Review
	if !reviewDecision.Approved && len(reviewDecision.Findings) == 0 {
		reviewDecision, err = (review.FinalReviewer{
			Agent:        m.Agent,
			ArtifactRoot: m.ArtifactRoot,
			RunID:        demandPackage.ID,
		}).Review(ctx, demandPackage, increment.Branch)
		if err != nil {
			return MacroResult{}, fmt.Errorf("final review: %w", err)
		}
	}
	result.Review = reviewDecision
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

func (m MacroOrchestrator) executeSpecs(ctx context.Context, runner flow.Runner, runID string, incrementBranch string, specs []domain.SpecRef, worktrees []SpecWorktree) ([]flow.StepResult, error) {
	var stepResults []flow.StepResult
	for i, spec := range specs {
		worktree := SpecWorktree{
			Branch:       "spec/" + runID + "/" + spec.ID,
			WorktreePath: filepath.Join(".dft", "worktrees", runID, spec.ID),
			SpecKitEnv: map[string]string{
				"GIT_BRANCH_NAME": "spec/" + runID + "/" + spec.ID,
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
		result, err := runner.Execute(ctx, BuildSpecKitLane(spec, worktree))
		stepResults = append(stepResults, result.Steps...)
		if err != nil {
			return stepResults, fmt.Errorf("run spec %s: %w", spec.ID, err)
		}
		if err := m.Worktrees.CompleteSpec(ctx, CompleteSpecRequest{
			SpecBranch:      worktree.Branch,
			IncrementBranch: incrementBranch,
		}); err != nil {
			return stepResults, fmt.Errorf("complete spec %s: %w", spec.ID, err)
		}
	}
	return stepResults, nil
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
