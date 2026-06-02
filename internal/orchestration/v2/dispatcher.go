package v2

import (
	"context"
	"fmt"

	"github.com/bocacorazon/dft/internal/domain"
	domainv2 "github.com/bocacorazon/dft/internal/domain/v2"
	"github.com/bocacorazon/dft/internal/ports"
)

// DispatcherInput contains everything the dispatcher needs to run.
type DispatcherInput struct {
	Specs           []domain.SpecRef
	LaneAssignments []domain.LaneAssignment
	RunID           string
	IncrementBranch string
	Registry        *ExecutorRegistry
	Agent           ports.AgentAdapter
	ArtifactRoot    string
}

// Dispatcher executes specs sequentially in WBS order, mapping each spec's
// lane assignment to an executor from the registry. No agent judgment —
// pure engine logic.
type Dispatcher struct {
	registry *ExecutorRegistry
}

// NewDispatcher creates a dispatcher backed by the given registry.
func NewDispatcher(registry *ExecutorRegistry) *Dispatcher {
	return &Dispatcher{registry: registry}
}

// Dispatch executes all specs and returns results. Fails fast on the first
// spec failure.
func (d *Dispatcher) Dispatch(ctx context.Context, input DispatcherInput) (domainv2.OrchestrationResult, error) {
	if len(input.Specs) == 0 {
		return domainv2.OrchestrationResult{}, fmt.Errorf("at least one spec is required")
	}

	laneBySpec := buildLaneMap(input.LaneAssignments)
	results := make([]domainv2.SpecResult, 0, len(input.Specs))

	for i, spec := range input.Specs {
		lane, ok := laneBySpec[spec.ID]
		if !ok {
			return domainv2.OrchestrationResult{
				DemandPackageID: input.RunID,
				IncrementBranch: input.IncrementBranch,
				SpecResults:     results,
			}, fmt.Errorf("spec %q has no lane assignment", spec.ID)
		}

		executor, ok := d.registry.Get(lane.Lane)
		if !ok {
			return domainv2.OrchestrationResult{
				DemandPackageID: input.RunID,
				IncrementBranch: input.IncrementBranch,
				SpecResults:     results,
			}, fmt.Errorf("unknown lane %q for spec %q (no executor registered)", lane.Lane, spec.ID)
		}

		worktreePath := fmt.Sprintf(".dft/worktrees/%s/%s", input.RunID, spec.ID)
		result, err := executor.Execute(ctx, ExecutorInput{
			Spec:            spec,
			RunID:           input.RunID,
			IncrementBranch: input.IncrementBranch,
			WorktreePath:    worktreePath,
			Agent:           input.Agent,
			ArtifactRoot:    input.ArtifactRoot,
		})
		if err != nil {
			results = append(results, domainv2.SpecResult{
				SpecID: spec.ID,
				Lane:   lane.Lane,
				Status: "failed",
			})
			return domainv2.OrchestrationResult{
				DemandPackageID: input.RunID,
				IncrementBranch: input.IncrementBranch,
				SpecResults:     results,
			}, fmt.Errorf("spec %s (lane %s) failed: %w", spec.ID, lane.Lane, err)
		}
		results = append(results, result)

		if result.Status == "failed" {
			return domainv2.OrchestrationResult{
				DemandPackageID: input.RunID,
				IncrementBranch: input.IncrementBranch,
				SpecResults:     results,
			}, fmt.Errorf("spec %s (lane %s) failed: stopping (%d of %d specs completed)", spec.ID, lane.Lane, i+1, len(input.Specs))
		}
	}

	return domainv2.OrchestrationResult{
		DemandPackageID: input.RunID,
		IncrementBranch: input.IncrementBranch,
		SpecResults:     results,
	}, nil
}

// buildLaneMap creates a spec ID → lane assignment lookup.
func buildLaneMap(assignments []domain.LaneAssignment) map[string]domain.LaneAssignment {
	m := make(map[string]domain.LaneAssignment, len(assignments))
	for _, a := range assignments {
		m[a.SpecID] = a
	}
	return m
}