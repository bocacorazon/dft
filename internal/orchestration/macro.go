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
	Agent        ports.AgentAdapter
	Worktrees    WorktreeManager
	Verifier     ports.Verifier
	ArtifactRoot string
	Review       domain.ReviewDecision
}

// MacroResult summarizes a completed macro-orchestration run.
type MacroResult struct {
	Increment        Increment                 `json:"increment"`
	SpecPlan         SpecPlanResult            `json:"spec_plan"`
	StepResults      []flow.StepResult         `json:"step_results"`
	Evaluation       domain.VerificationResult `json:"evaluation"`
	Review           domain.ReviewDecision     `json:"review"`
	FinalMergeTarget string                    `json:"final_merge_target"`
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

	runner := flow.Runner{Agent: m.Agent, ArtifactRoot: m.ArtifactRoot, RunID: demandPackage.ID}
	var stepResults []flow.StepResult
	for _, spec := range specPlan.WBS.Specs {
		result, err := runner.Execute(ctx, flow.Definition{Steps: []flow.Step{{
			ID:        "spec-" + spec.ID,
			Type:      flow.StepAgent,
			AgentName: "dft-intake.agent.md",
			Prompt:    "Execute spec: " + spec.Description,
			Demand:    spec.Description,
		}}})
		stepResults = append(stepResults, result.Steps...)
		if err != nil {
			return MacroResult{}, fmt.Errorf("run spec %s: %w", spec.ID, err)
		}
		if err := m.Worktrees.CompleteSpec(ctx, CompleteSpecRequest{
			SpecBranch:      "spec/" + demandPackage.ID + "/" + spec.ID,
			IncrementBranch: increment.Branch,
		}); err != nil {
			return MacroResult{}, fmt.Errorf("complete spec %s: %w", spec.ID, err)
		}
	}

	evaluation, err := (review.Evaluator{
		Verifier:     m.Verifier,
		ArtifactRoot: m.ArtifactRoot,
		RunID:        demandPackage.ID,
	}).Evaluate(ctx, []domain.Check{
		{ID: "wbs", Kind: domain.CheckFileExists, Args: []string{filepath.Join(".dft", "runs", demandPackage.ID, "design", "wbs.json")}},
		{ID: "lane-assignments", Kind: domain.CheckFileExists, Args: []string{filepath.Join(".dft", "runs", demandPackage.ID, "design", "lane-assignments.json")}},
	})
	if err != nil {
		return MacroResult{}, fmt.Errorf("evaluate increment: %w", err)
	}

	reviewDecision := m.Review
	if !reviewDecision.Approved && len(reviewDecision.Findings) == 0 {
		reviewDecision.Approved = true
	}
	if err := m.Worktrees.CompleteIncrement(ctx, CompleteIncrementRequest{
		IncrementBranch: increment.Branch,
		DefaultBranch:   increment.DefaultBranch,
		Evaluation:      evaluation,
		Review:          reviewDecision,
	}); err != nil {
		return MacroResult{}, fmt.Errorf("complete increment: %w", err)
	}

	result := MacroResult{
		Increment:        increment,
		SpecPlan:         specPlan,
		StepResults:      stepResults,
		Evaluation:       evaluation,
		Review:           reviewDecision,
		FinalMergeTarget: increment.DefaultBranch,
	}
	if err := writeMacroResult(m.ArtifactRoot, demandPackage.ID, result); err != nil {
		return MacroResult{}, err
	}
	return result, nil
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
