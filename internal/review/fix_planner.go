package review

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/agentjson"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// FixPlanner turns failed evaluation findings into actionable remediation.
type FixPlanner struct {
	Agent        ports.AgentAdapter
	ArtifactRoot string
	RunID        string
}

// Plan writes `.dft/runs/<run-id>/fix-plan/wbs-amendment.json`.
func (p FixPlanner) Plan(ctx context.Context, demandPackage domain.DemandPackage, result domain.VerificationResult) (domain.WBSAmendment, error) {
	if p.Agent == nil {
		return domain.WBSAmendment{}, fmt.Errorf("agent adapter is required")
	}
	if p.RunID == "" {
		return domain.WBSAmendment{}, fmt.Errorf("run id is required")
	}
	if len(result.Findings) == 0 {
		return domain.WBSAmendment{}, fmt.Errorf("fix planner requires failed evaluation findings")
	}
	findings, err := json.MarshalIndent(result.Findings, "", "  ")
	if err != nil {
		return domain.WBSAmendment{}, fmt.Errorf("encode failed findings: %w", err)
	}
	response, err := p.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-fix-planner.agent.md",
		Prompt:    "Plan WBS amendments for failed evaluation or review findings.\n\nFindings:\n" + string(findings),
		Demand:    demandPackage.RawDemand,
		RunID:     p.RunID,
	})
	if err != nil {
		return domain.WBSAmendment{}, fmt.Errorf("invoke fix planner: %w", err)
	}

	var amendment domain.WBSAmendment
	if err := agentjson.DecodeFirst(response.Raw, &amendment); err != nil {
		return domain.WBSAmendment{}, fmt.Errorf("parse fix planner output: %w", err)
	}
	if len(amendment.Findings) == 0 {
		amendment.Findings = result.Findings
	}
	if amendment.DemandPackageID == "" {
		amendment.DemandPackageID = demandPackage.ID
	}
	if err := amendment.Validate(); err != nil {
		return domain.WBSAmendment{}, fmt.Errorf("validate WBS amendment: %w", err)
	}
	if err := writeJSON(filepath.Join(p.ArtifactRoot, ".dft", "runs", p.RunID, "fix-plan", "wbs-amendment.json"), amendment); err != nil {
		return domain.WBSAmendment{}, err
	}
	return amendment, nil
}
