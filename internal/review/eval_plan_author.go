package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// EvalPlanAuthor asks a distinct agent to produce deterministic checks.
type EvalPlanAuthor struct {
	Agent        ports.AgentAdapter
	ArtifactRoot string
	RunID        string
}

// Author creates and persists `.dft/runs/<run-id>/eval-plan.json`.
func (a EvalPlanAuthor) Author(ctx context.Context, demandPackage domain.DemandPackage) (domain.EvaluationPlan, error) {
	if a.Agent == nil {
		return domain.EvaluationPlan{}, fmt.Errorf("agent adapter is required")
	}
	if a.RunID == "" {
		return domain.EvaluationPlan{}, fmt.Errorf("run id is required")
	}
	if err := demandPackage.Validate(); err != nil {
		return domain.EvaluationPlan{}, fmt.Errorf("validate demand package: %w", err)
	}
	response, err := a.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-eval-plan-author.agent.md",
		Prompt:    "Author deterministic evaluation plan for demand package: " + demandPackage.Title,
		Demand:    demandPackage.RawDemand,
		RunID:     a.RunID,
	})
	if err != nil {
		return domain.EvaluationPlan{}, fmt.Errorf("invoke eval plan author: %w", err)
	}

	var plan domain.EvaluationPlan
	if err := json.Unmarshal([]byte(response.Raw), &plan); err != nil {
		return domain.EvaluationPlan{}, fmt.Errorf("parse eval plan author output: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return domain.EvaluationPlan{}, fmt.Errorf("validate eval plan: %w", err)
	}
	if err := writeJSON(filepath.Join(a.ArtifactRoot, ".dft", "runs", a.RunID, "eval-plan.json"), plan); err != nil {
		return domain.EvaluationPlan{}, err
	}
	return plan, nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
