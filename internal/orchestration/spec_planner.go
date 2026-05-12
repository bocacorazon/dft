package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// SpecPlanner creates WBS/lane design artifacts and spec worktrees.
type SpecPlanner struct {
	Agent        ports.AgentAdapter
	Worktrees    WorktreeManager
	ArtifactRoot string
}

// SpecPlanResult captures the design output needed by the spec loop.
type SpecPlanResult struct {
	WBS             domain.WBS
	LaneAssignments []domain.LaneAssignment
	Worktrees       []SpecWorktree
}

// PlanSpecs turns a demand package into specs, lanes, and per-spec worktrees.
func (p SpecPlanner) PlanSpecs(ctx context.Context, demandPackage domain.DemandPackage, incrementBranch string) (SpecPlanResult, error) {
	if err := demandPackage.Validate(); err != nil {
		return SpecPlanResult{}, fmt.Errorf("validate demand package: %w", err)
	}
	if p.Agent == nil {
		return SpecPlanResult{}, fmt.Errorf("agent adapter is required")
	}
	if err := validateRef("increment branch", incrementBranch); err != nil {
		return SpecPlanResult{}, err
	}

	designDir := filepath.Join(p.ArtifactRoot, ".dft", "runs", demandPackage.ID, "design")
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		return SpecPlanResult{}, fmt.Errorf("create design artifact directory: %w", err)
	}

	wbs, err := p.buildWBS(ctx, demandPackage)
	if err != nil {
		return SpecPlanResult{}, err
	}
	if err := writeJSON(filepath.Join(designDir, "wbs.json"), wbs); err != nil {
		return SpecPlanResult{}, err
	}

	assignments, err := p.selectLanes(ctx, demandPackage, wbs)
	if err != nil {
		return SpecPlanResult{}, err
	}
	if err := writeJSON(filepath.Join(designDir, "lane-assignments.json"), assignments); err != nil {
		return SpecPlanResult{}, err
	}

	worktrees := make([]SpecWorktree, 0, len(wbs.Specs))
	for _, spec := range wbs.Specs {
		worktree, err := p.Worktrees.BeginSpec(ctx, SpecRequest{
			RunID:           demandPackage.ID,
			SpecID:          spec.ID,
			IncrementBranch: incrementBranch,
		})
		if err != nil {
			return SpecPlanResult{}, err
		}
		worktrees = append(worktrees, worktree)
	}

	return SpecPlanResult{WBS: wbs, LaneAssignments: assignments, Worktrees: worktrees}, nil
}

func (p SpecPlanner) buildWBS(ctx context.Context, demandPackage domain.DemandPackage) (domain.WBS, error) {
	response, err := p.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-wbs-builder.agent.md",
		Prompt:    "Build WBS for demand package: " + demandPackage.Title,
		Demand:    demandPackage.RawDemand,
		RunID:     demandPackage.ID,
	})
	if err != nil {
		return domain.WBS{}, fmt.Errorf("invoke WBS builder: %w", err)
	}

	var wbs domain.WBS
	if err := json.Unmarshal([]byte(response.Raw), &wbs); err != nil {
		return domain.WBS{}, fmt.Errorf("parse WBS builder output: %w", err)
	}
	if err := wbs.Validate(); err != nil {
		return domain.WBS{}, fmt.Errorf("validate WBS: %w", err)
	}
	return wbs, nil
}

func (p SpecPlanner) selectLanes(ctx context.Context, demandPackage domain.DemandPackage, wbs domain.WBS) ([]domain.LaneAssignment, error) {
	response, err := p.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-lane-selector.agent.md",
		Prompt:    "Select lanes for WBS: " + demandPackage.Title,
		Demand:    demandPackage.RawDemand,
		RunID:     demandPackage.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke lane selector: %w", err)
	}

	var assignments []domain.LaneAssignment
	if err := json.Unmarshal([]byte(response.Raw), &assignments); err != nil {
		return nil, fmt.Errorf("parse lane selector output: %w", err)
	}
	if err := domain.ValidateLaneAssignments(assignments); err != nil {
		return nil, fmt.Errorf("validate lane assignments: %w", err)
	}
	specIDs := make(map[string]struct{}, len(wbs.Specs))
	for _, spec := range wbs.Specs {
		specIDs[spec.ID] = struct{}{}
	}
	for _, assignment := range assignments {
		if _, ok := specIDs[assignment.SpecID]; !ok {
			return nil, fmt.Errorf("lane assignment references unknown spec %q", assignment.SpecID)
		}
	}
	return assignments, nil
}

func writeJSON(path string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
