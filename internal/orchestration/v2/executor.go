// Package v2 implements the pluggable executor system for dft's Build phase.
//
// Executors map lane names to concrete execution plans. When the dispatcher
// encounters a spec with lane assignment "speckit", it looks up the "speckit"
// executor from the registry and delegates execution to it.
//
// Register new executors via registry.Register(name, constructor).
package v2

import (
	"context"

	"github.com/bocacorazon/dft/internal/domain"
	domainv2 "github.com/bocacorazon/dft/internal/domain/v2"
	"github.com/bocacorazon/dft/internal/ports"
)

// Executor runs one spec using a specific lane (execution plan).
// Lane names map directly to executors: "speckit" → SpeckitExecutor,
// "direct" → DirectExecutor, "stub" → StubExecutor, etc.
type Executor interface {
	// Name returns the lane name this executor handles.
	Name() string

	// Execute runs one spec and returns the result.
	Execute(ctx context.Context, input ExecutorInput) (domainv2.SpecResult, error)
}

// ExecutorInput is what every executor receives.
type ExecutorInput struct {
	Spec            domain.SpecRef
	RunID           string
	IncrementBranch string
	WorktreePath    string
	Agent           ports.AgentAdapter
	ArtifactRoot    string
}

// ExecutorRegistry maps lane names to executor constructors.
// Constructors are called once per spec execution, producing a fresh Executor each time.
type ExecutorRegistry struct {
	executors map[string]func() Executor
}

// NewExecutorRegistry creates an empty registry.
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{executors: make(map[string]func() Executor)}
}

// Register adds a lane name → executor constructor mapping.
func (r *ExecutorRegistry) Register(name string, ctor func() Executor) {
	r.executors[name] = ctor
}

// Get returns a fresh executor instance for the given lane name.
func (r *ExecutorRegistry) Get(name string) (Executor, bool) {
	ctor, ok := r.executors[name]
	if !ok {
		return nil, false
	}
	return ctor(), true
}

// Names returns all registered lane names.
func (r *ExecutorRegistry) Names() []string {
	names := make([]string, 0, len(r.executors))
	for name := range r.executors {
		names = append(names, name)
	}
	return names
}