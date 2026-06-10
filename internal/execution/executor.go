package execution

import (
	"context"

	"github.com/bocacorazon/dft/internal/domain"
)

// Executor is the inner loop: it runs one spec's whole flow and owns its state.
// The outer Orchestrator dispatches each spec to the executor selected for its
// lane, then asks that executor whether the spec is already complete before
// running it again on resume.
type Executor interface {
	// Execute runs one spec to completion and records its result.
	Execute(ctx context.Context, runID string, specID string, request domain.ExecutionRequest) (domain.ExecutionResult, error)
	// Status reports the spec's completeness derived from artifact truth, so the
	// orchestrator can skip finished specs when resuming an interrupted run.
	Status(ctx context.Context, runID string, specID string, request domain.ExecutionRequest) (domain.ExecutionStatus, error)
}

// ExecutorRegistry maps lane names to executors and resolves the executor for a
// spec. An empty lane resolves to the default executor.
type ExecutorRegistry struct {
	byLane  map[string]Executor
	defawlt Executor
}

// NewRegistry creates a registry whose default executor handles specs with no
// explicit lane (and any lane that is not separately registered).
func NewRegistry(defaultExecutor Executor) *ExecutorRegistry {
	return &ExecutorRegistry{
		byLane:  make(map[string]Executor),
		defawlt: defaultExecutor,
	}
}

// Register binds a lane name to an executor.
func (r *ExecutorRegistry) Register(lane string, executor Executor) {
	if r.byLane == nil {
		r.byLane = make(map[string]Executor)
	}
	r.byLane[lane] = executor
}

// For returns the executor selected for the given lane, falling back to the
// default executor when the lane is empty or unregistered.
func (r *ExecutorRegistry) For(lane string) (Executor, bool) {
	if r == nil {
		return nil, false
	}
	if executor, ok := r.byLane[lane]; ok && lane != "" {
		return executor, true
	}
	if r.defawlt != nil {
		return r.defawlt, true
	}
	return nil, false
}
