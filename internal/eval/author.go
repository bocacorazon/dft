package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/agentjson"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// AuthorInput is the source-blind context allowed for eval-plan authoring.
type AuthorInput struct {
	DemandPackage    domain.DemandPackage       `json:"demand_package"`
	WBS              domain.WBS                 `json:"wbs"`
	SurfaceContract  domain.EvalSurfaceContract `json:"surface_contract"`
	ArtifactManifest domain.ArtifactManifest    `json:"artifact_manifest"`
	Readiness        domain.EvalReady           `json:"readiness"`
	StepCatalog      []string                   `json:"step_catalog,omitempty"`
}

// ArtifactOnlyPlanAuthor asks an independent agent for a source-blind eval plan.
type ArtifactOnlyPlanAuthor struct {
	Agent        ports.AgentAdapter
	ArtifactRoot string
	RunID        string
}

// Author writes `.dft/runs/<run-id>/eval/eval-plan.json`.
func (a ArtifactOnlyPlanAuthor) Author(ctx context.Context, input AuthorInput) (domain.EvalPlan, error) {
	if a.Agent == nil {
		return domain.EvalPlan{}, fmt.Errorf("agent adapter is required")
	}
	if a.RunID == "" {
		return domain.EvalPlan{}, fmt.Errorf("run id is required")
	}
	if err := input.DemandPackage.Validate(); err != nil {
		return domain.EvalPlan{}, fmt.Errorf("validate demand package: %w", err)
	}
	if err := input.WBS.Validate(); err != nil {
		return domain.EvalPlan{}, fmt.Errorf("validate WBS: %w", err)
	}
	if err := input.SurfaceContract.Validate(); err != nil {
		return domain.EvalPlan{}, fmt.Errorf("validate surface contract: %w", err)
	}
	if err := input.ArtifactManifest.Validate(); err != nil {
		return domain.EvalPlan{}, fmt.Errorf("validate artifact manifest: %w", err)
	}
	promptBody, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return domain.EvalPlan{}, fmt.Errorf("encode eval author input: %w", err)
	}
	response, err := a.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-eval-plan-author.agent.md",
		Prompt: "Author a hidden, artifact-only BDD eval plan from this source-blind context. " +
			"Use only declared surfaces and artifacts; do not ask for or infer implementation source.\n\n" + string(promptBody),
		Demand: input.DemandPackage.RawDemand,
		RunID:  a.RunID,
	})
	if err != nil {
		return domain.EvalPlan{}, fmt.Errorf("invoke eval plan author: %w", err)
	}

	var plan domain.EvalPlan
	if err := agentjson.DecodeFirst(response.Raw, &plan); err != nil {
		return domain.EvalPlan{}, fmt.Errorf("parse eval plan author output: %w", err)
	}
	if plan.DemandPackageID == "" {
		plan.DemandPackageID = input.DemandPackage.ID
	}
	defaultHidden(&plan)
	if err := plan.Validate(); err != nil {
		return domain.EvalPlan{}, fmt.Errorf("validate eval plan: %w", err)
	}
	if err := writeJSON(filepath.Join(a.ArtifactRoot, ".dft", "runs", a.RunID, "eval", "eval-plan.json"), plan); err != nil {
		return domain.EvalPlan{}, err
	}
	return plan, nil
}

func defaultHidden(plan *domain.EvalPlan) {
	for index := range plan.Packs {
		if plan.Packs[index].Visibility == "" {
			plan.Packs[index].Visibility = domain.EvalVisibilityHidden
		}
	}
}
