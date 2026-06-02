package v2

import (
	"context"
	"fmt"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
	domainv2 "github.com/bocacorazon/dft/internal/domain/v2"
)

func TestDispatcher_SingleSpec(t *testing.T) {
	registry := NewExecutorRegistry()
	registry.Register("stub", func() Executor { return &StubExecutor{} })

	specs := []domain.SpecRef{
		{ID: "spec-1", Description: "first spec", AcceptanceCriteria: []string{"works"}},
	}
	laneAssignments := []domain.LaneAssignment{
		{SpecID: "spec-1", Lane: "stub", Rationale: "smoke test"},
	}

	d := NewDispatcher(registry)
	result, err := d.Dispatch(context.Background(), DispatcherInput{
		Specs:           specs,
		LaneAssignments: laneAssignments,
		RunID:           "test-run",
		IncrementBranch: "increment/test-run",
		ArtifactRoot:    ".",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.SpecResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.SpecResults))
	}
	r := result.SpecResults[0]
	if r.SpecID != "spec-1" {
		t.Fatalf("expected spec-1, got %s", r.SpecID)
	}
	if r.Lane != "stub" {
		t.Fatalf("expected lane stub, got %s", r.Lane)
	}
	if r.Status != "completed" {
		t.Fatalf("expected completed, got %s", r.Status)
	}
}

func TestDispatcher_MultiSpec(t *testing.T) {
	registry := NewExecutorRegistry()
	registry.Register("stub", func() Executor { return &StubExecutor{} })

	specs := []domain.SpecRef{
		{ID: "spec-1", Description: "first", AcceptanceCriteria: []string{"a"}},
		{ID: "spec-2", Description: "second", AcceptanceCriteria: []string{"b"}},
		{ID: "spec-3", Description: "third", AcceptanceCriteria: []string{"c"}},
	}
	laneAssignments := []domain.LaneAssignment{
		{SpecID: "spec-1", Lane: "stub"},
		{SpecID: "spec-2", Lane: "stub"},
		{SpecID: "spec-3", Lane: "stub"},
	}

	d := NewDispatcher(registry)
	result, err := d.Dispatch(context.Background(), DispatcherInput{
		Specs:           specs,
		LaneAssignments: laneAssignments,
		RunID:           "test-run",
		IncrementBranch: "increment/test-run",
		ArtifactRoot:    ".",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.SpecResults) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.SpecResults))
	}
	for i, r := range result.SpecResults {
		expectedID := specs[i].ID
		if r.SpecID != expectedID {
			t.Fatalf("result %d: expected spec %s, got %s", i, expectedID, r.SpecID)
		}
		if r.Status != "completed" {
			t.Fatalf("result %d: expected completed, got %s", i, r.Status)
		}
	}
}

func TestDispatcher_UnknownLane(t *testing.T) {
	registry := NewExecutorRegistry()

	specs := []domain.SpecRef{
		{ID: "spec-1", Description: "test", AcceptanceCriteria: []string{"a"}},
	}
	laneAssignments := []domain.LaneAssignment{
		{SpecID: "spec-1", Lane: "nonexistent"},
	}

	d := NewDispatcher(registry)
	_, err := d.Dispatch(context.Background(), DispatcherInput{
		Specs:           specs,
		LaneAssignments: laneAssignments,
		RunID:           "test-run",
		IncrementBranch: "increment/test-run",
		ArtifactRoot:    ".",
	})
	if err == nil {
		t.Fatal("expected error for unknown lane, got nil")
	}
}

func TestDispatcher_MissingLaneAssignment(t *testing.T) {
	registry := NewExecutorRegistry()
	registry.Register("stub", func() Executor { return &StubExecutor{} })

	specs := []domain.SpecRef{
		{ID: "spec-1", Description: "test", AcceptanceCriteria: []string{"a"}},
	}
	// no lane assignment for spec-1

	d := NewDispatcher(registry)
	_, err := d.Dispatch(context.Background(), DispatcherInput{
		Specs:           specs,
		LaneAssignments: nil,
		RunID:           "test-run",
		IncrementBranch: "increment/test-run",
		ArtifactRoot:    ".",
	})
	if err == nil {
		t.Fatal("expected error for missing lane assignment, got nil")
	}
}

func TestDispatcher_FailFast(t *testing.T) {
	registry := NewExecutorRegistry()
	registry.Register("failing", func() Executor {
		return &failingExecutor{name: "failing"}
	})
	registry.Register("stub", func() Executor { return &StubExecutor{} })

	specs := []domain.SpecRef{
		{ID: "spec-1", Description: "will fail", AcceptanceCriteria: []string{"a"}},
		{ID: "spec-2", Description: "should not run", AcceptanceCriteria: []string{"b"}},
	}
	laneAssignments := []domain.LaneAssignment{
		{SpecID: "spec-1", Lane: "failing"},
		{SpecID: "spec-2", Lane: "stub"},
	}

	d := NewDispatcher(registry)
	result, err := d.Dispatch(context.Background(), DispatcherInput{
		Specs:           specs,
		LaneAssignments: laneAssignments,
		RunID:           "test-run",
		IncrementBranch: "increment/test-run",
		ArtifactRoot:    ".",
	})
	if err == nil {
		t.Fatal("expected error from failing executor, got nil")
	}
	if len(result.SpecResults) != 1 {
		t.Fatalf("expected 1 result before fail-fast, got %d", len(result.SpecResults))
	}
}

func TestDispatcher_EmptySpecs(t *testing.T) {
	registry := NewExecutorRegistry()

	d := NewDispatcher(registry)
	_, err := d.Dispatch(context.Background(), DispatcherInput{
		Specs:           nil,
		RunID:           "test-run",
		IncrementBranch: "increment/test-run",
		ArtifactRoot:    ".",
	})
	if err == nil {
		t.Fatal("expected error for empty specs, got nil")
	}
}

// failingExecutor always returns an error.
type failingExecutor struct {
	name string
}

func (f *failingExecutor) Name() string                    { return f.name }
func (f *failingExecutor) Execute(_ context.Context, _ ExecutorInput) (domainv2.SpecResult, error) {
	return domainv2.SpecResult{}, fmt.Errorf("simulated failure")
}

var _ Executor = (*failingExecutor)(nil)