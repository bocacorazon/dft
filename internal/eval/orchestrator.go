package eval

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// Orchestrator runs the artifact-only eval phase once artifacts are collected.
type Orchestrator struct {
	Agent        ports.AgentAdapter
	Verifier     ports.Verifier
	ArtifactRoot string
	RunID        string
	HTTPClient   *http.Client
}

// Result captures authored plan and executed verdict for one eval phase.
type Result struct {
	Ready  domain.EvalReady  `json:"ready"`
	Plan   domain.EvalPlan   `json:"plan,omitempty"`
	Result domain.EvalResult `json:"result"`
}

// Run performs readiness binding, source-blind plan authoring, and BDD execution.
func (o Orchestrator) Run(ctx context.Context, input AuthorInput) (Result, error) {
	if o.RunID == "" {
		return Result{}, fmt.Errorf("run id is required")
	}
	ready, err := (ReadinessGate{
		RootDir:    o.ArtifactRoot,
		HTTPClient: o.HTTPClient,
	}).Check(ctx, input.SurfaceContract, input.ArtifactManifest)
	if err != nil {
		return Result{}, err
	}
	if err := writeJSON(filepath.Join(o.ArtifactRoot, ".dft", "runs", o.RunID, "eval", "eval-ready.json"), ready); err != nil {
		return Result{}, err
	}
	if ready.Status == domain.EvalStatusBlocked {
		blocked := domain.EvalResult{
			Status:    domain.EvalStatusBlocked,
			Readiness: &ready,
			Findings:  append([]domain.Finding(nil), ready.Findings...),
		}
		if err := writeJSON(filepath.Join(o.ArtifactRoot, ".dft", "runs", o.RunID, "eval", "evaluation.json"), blocked); err != nil {
			return Result{}, err
		}
		return Result{Ready: ready, Result: blocked}, nil
	}
	input.Readiness = ready
	plan, err := (ArtifactOnlyPlanAuthor{
		Agent:        o.Agent,
		ArtifactRoot: o.ArtifactRoot,
		RunID:        o.RunID,
	}).Author(ctx, input)
	if err != nil {
		return Result{}, err
	}
	evalResult, err := (Executor{
		RootDir:    o.ArtifactRoot,
		HTTPClient: o.HTTPClient,
		Verifier:   o.Verifier,
	}).Execute(ctx, o.RunID, ready, plan)
	if err != nil {
		return Result{}, err
	}
	return Result{Ready: ready, Plan: plan, Result: evalResult}, nil
}
