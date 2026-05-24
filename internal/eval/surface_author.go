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

// SurfaceContractAuthor asks a design-time agent to declare where eval will test.
type SurfaceContractAuthor struct {
	Agent        ports.AgentAdapter
	ArtifactRoot string
	RunID        string
}

// Author writes `.dft/runs/<run-id>/design/eval-surfaces.json`.
func (a SurfaceContractAuthor) Author(ctx context.Context, demandPackage domain.DemandPackage, wbs domain.WBS) (domain.EvalSurfaceContract, error) {
	if a.Agent == nil {
		return domain.EvalSurfaceContract{}, fmt.Errorf("agent adapter is required")
	}
	if a.RunID == "" {
		return domain.EvalSurfaceContract{}, fmt.Errorf("run id is required")
	}
	if err := demandPackage.Validate(); err != nil {
		return domain.EvalSurfaceContract{}, fmt.Errorf("validate demand package: %w", err)
	}
	if err := wbs.Validate(); err != nil {
		return domain.EvalSurfaceContract{}, fmt.Errorf("validate WBS: %w", err)
	}
	input := struct {
		DemandPackage domain.DemandPackage `json:"demand_package"`
		WBS           domain.WBS           `json:"wbs"`
	}{
		DemandPackage: demandPackage,
		WBS:           wbs,
	}
	content, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return domain.EvalSurfaceContract{}, fmt.Errorf("encode surface author input: %w", err)
	}
	response, err := a.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-eval-surface-author.agent.md",
		Prompt: "Author the evaluation surface contract during solution design. " +
			"Declare where artifact-only eval will test each acceptance criterion. Do not inspect source code.\n\n" + string(content),
		Demand: demandPackage.RawDemand,
		RunID:  a.RunID,
	})
	if err != nil {
		return domain.EvalSurfaceContract{}, fmt.Errorf("invoke eval surface author: %w", err)
	}

	var contract domain.EvalSurfaceContract
	if err := agentjson.DecodeFirst(response.Raw, &contract); err != nil {
		return domain.EvalSurfaceContract{}, fmt.Errorf("parse eval surface author output: %w", err)
	}
	if contract.DemandPackageID == "" {
		contract.DemandPackageID = demandPackage.ID
	}
	if err := contract.Validate(); err != nil {
		return domain.EvalSurfaceContract{}, fmt.Errorf("validate eval surface contract: %w", err)
	}
	if err := writeJSON(filepath.Join(a.ArtifactRoot, ".dft", "runs", a.RunID, "design", "eval-surfaces.json"), contract); err != nil {
		return domain.EvalSurfaceContract{}, err
	}
	return contract, nil
}
