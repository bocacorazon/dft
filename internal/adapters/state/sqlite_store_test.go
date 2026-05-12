package state

import (
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestSQLiteStorePersistsRunsQueueAndSteps(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), ".dft", "state.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	run := domain.RunManifest{
		ID:        "run-123",
		Status:    domain.RunRunning,
		Adapter:   "stub",
		RawDemand: "Build durable state",
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := store.Enqueue("job-1", run.ID); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if err := store.SaveStep(domain.StepRecord{
		RunID:  run.ID,
		StepID: "intent",
		Status: domain.StepCommitted,
		Commit: "abc123",
	}); err != nil {
		t.Fatalf("SaveStep returned error: %v", err)
	}

	loaded, err := store.Load(run.ID)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded != run {
		t.Fatalf("loaded run = %#v, want %#v", loaded, run)
	}

	next, err := store.NextQueued()
	if err != nil {
		t.Fatalf("NextQueued returned error: %v", err)
	}
	if next.RunID != run.ID || next.Status != domain.JobQueued {
		t.Fatalf("next job = %#v, want queued run", next)
	}

	steps, err := store.ListSteps(run.ID)
	if err != nil {
		t.Fatalf("ListSteps returned error: %v", err)
	}
	if len(steps) != 1 || steps[0].Commit != "abc123" {
		t.Fatalf("steps = %#v, want committed step", steps)
	}
}

func TestSQLiteStoreReconcilesCommittedStepAfterCrash(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), ".dft", "state.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	if err := store.Save(domain.RunManifest{ID: "run-123", Status: domain.RunRunning}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := store.SaveStep(domain.StepRecord{RunID: "run-123", StepID: "plan", Status: domain.StepCommitting, Commit: "def456"}); err != nil {
		t.Fatalf("SaveStep returned error: %v", err)
	}

	if err := store.ReconcileCommittedSteps("run-123", []domain.CommitStep{{StepID: "plan", Commit: "def456"}}); err != nil {
		t.Fatalf("ReconcileCommittedSteps returned error: %v", err)
	}

	steps, err := store.ListSteps("run-123")
	if err != nil {
		t.Fatalf("ListSteps returned error: %v", err)
	}
	if steps[0].Status != domain.StepCommitted {
		t.Fatalf("step status = %q, want committed", steps[0].Status)
	}
}
