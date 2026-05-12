package review

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/agentjson"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// FinalReviewer invokes the final review gate before increment merge.
type FinalReviewer struct {
	Agent        ports.AgentAdapter
	ArtifactRoot string
	RunID        string
}

// Review writes `.dft/runs/<run-id>/review/final-review.json`.
func (r FinalReviewer) Review(ctx context.Context, demandPackage domain.DemandPackage, incrementBranch string) (domain.ReviewDecision, error) {
	if r.Agent == nil {
		return domain.ReviewDecision{}, fmt.Errorf("agent adapter is required")
	}
	if r.RunID == "" {
		return domain.ReviewDecision{}, fmt.Errorf("run id is required")
	}
	response, err := r.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-review.agent.md",
		Prompt:    "Perform final code review for increment branch " + incrementBranch,
		Demand:    demandPackage.RawDemand,
		RunID:     r.RunID,
	})
	if err != nil {
		return domain.ReviewDecision{}, fmt.Errorf("invoke final reviewer: %w", err)
	}

	var decision domain.ReviewDecision
	if err := agentjson.DecodeFirst(response.Raw, &decision); err != nil {
		return domain.ReviewDecision{}, fmt.Errorf("parse final review output: %w", err)
	}
	if !decision.Approved && len(decision.Findings) == 0 {
		return domain.ReviewDecision{}, fmt.Errorf("final review rejected without findings")
	}
	if err := writeJSON(filepath.Join(r.ArtifactRoot, ".dft", "runs", r.RunID, "review", "final-review.json"), decision); err != nil {
		return domain.ReviewDecision{}, err
	}
	return decision, nil
}
