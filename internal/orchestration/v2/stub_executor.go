package v2

import (
	"context"

	"github.com/bocacorazon/dft/internal/domain"
	domainv2 "github.com/bocacorazon/dft/internal/domain/v2"
)

// StubExecutor returns deterministic completed results.
// Use it for smoke tests and dispatcher validation without real agents.
type StubExecutor struct{}

func (s *StubExecutor) Name() string { return "stub" }

func (s *StubExecutor) Execute(_ context.Context, input ExecutorInput) (domainv2.SpecResult, error) {
	return domainv2.SpecResult{
		SpecID: input.Spec.ID,
		Lane:   "stub",
		Status: "completed",
		Branch: input.IncrementBranch,
		Artifacts: []domain.ArtifactRef{
			{ID: input.Spec.ID + "-artifact", Kind: domain.ArtifactFile, Path: input.WorktreePath},
		},
	}, nil
}

// Ensure StubExecutor implements Executor.
var _ Executor = (*StubExecutor)(nil)