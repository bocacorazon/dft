package state

import (
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestSQLiteStorePersistsRunsAndQueue(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), ".dft", "state.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	run := domain.RunManifest{
		ID:           "run-123",
		Status:       domain.RunRunning,
		Adapter:      "stub",
		RawIncrement: "Build durable state",
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := store.Enqueue("job-1", run.ID); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
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
}

func TestSQLiteStorePersistsInboxEntries(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), ".dft", "state.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	if err := store.Save(domain.RunManifest{ID: "run-123", Status: domain.RunRunning}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := store.SaveInboxEntry(domain.InboxEntry{
		ID:      "entry-1",
		RunID:   "run-123",
		StepID:  "approval",
		Status:  "open",
		Message: "approval required",
	}); err != nil {
		t.Fatalf("SaveInboxEntry returned error: %v", err)
	}

	entries, err := store.ListInboxEntries("run-123")
	if err != nil {
		t.Fatalf("ListInboxEntries returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "approval required" {
		t.Fatalf("entries = %#v, want saved inbox entry", entries)
	}
}
