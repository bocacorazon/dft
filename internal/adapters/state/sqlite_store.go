package state

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore persists jobs, runs, and step recovery metadata in .dft/state.db.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLiteStore opens or creates a sqlite-backed dft state store.
func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite state: %w", err)
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close closes the underlying sqlite connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			adapter TEXT NOT NULL,
			raw_demand TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT NOT NULL UNIQUE,
			run_id TEXT NOT NULL,
			status TEXT NOT NULL,
			created_order INTEGER PRIMARY KEY AUTOINCREMENT
		)`,
		`CREATE TABLE IF NOT EXISTS steps (
			run_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			status TEXT NOT NULL,
			commit_sha TEXT,
			PRIMARY KEY (run_id, step_id)
		)`,
		`CREATE TABLE IF NOT EXISTS inbox_entries (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			step_id TEXT,
			status TEXT NOT NULL,
			message TEXT NOT NULL
		)`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate sqlite state: %w", err)
		}
	}
	return nil
}

// SaveInboxEntry persists a human-facing escalation or manual gate.
func (s *SQLiteStore) SaveInboxEntry(entry domain.InboxEntry) error {
	if entry.ID == "" || entry.RunID == "" {
		return fmt.Errorf("inbox entry id and run id are required")
	}
	if entry.Status == "" {
		entry.Status = "open"
	}
	_, err := s.db.Exec(
		`INSERT INTO inbox_entries (id, run_id, step_id, status, message) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET run_id=excluded.run_id, step_id=excluded.step_id, status=excluded.status, message=excluded.message`,
		entry.ID, entry.RunID, entry.StepID, entry.Status, entry.Message,
	)
	if err != nil {
		return fmt.Errorf("save inbox entry: %w", err)
	}
	return nil
}

// ListInboxEntries returns durable inbox entries for a run.
func (s *SQLiteStore) ListInboxEntries(runID string) ([]domain.InboxEntry, error) {
	rows, err := s.db.Query(`SELECT id, run_id, COALESCE(step_id, ''), status, message FROM inbox_entries WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list inbox entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.InboxEntry
	for rows.Next() {
		var entry domain.InboxEntry
		if err := rows.Scan(&entry.ID, &entry.RunID, &entry.StepID, &entry.Status, &entry.Message); err != nil {
			return nil, fmt.Errorf("scan inbox entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbox entries: %w", err)
	}
	return entries, nil
}

// Save writes a run manifest.
func (s *SQLiteStore) Save(manifest domain.RunManifest) error {
	if manifest.ID == "" {
		return fmt.Errorf("run id is required")
	}
	_, err := s.db.Exec(
		`INSERT INTO runs (id, status, adapter, raw_demand) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET status=excluded.status, adapter=excluded.adapter, raw_demand=excluded.raw_demand`,
		manifest.ID, manifest.Status, manifest.Adapter, manifest.RawDemand,
	)
	if err != nil {
		return fmt.Errorf("save run: %w", err)
	}
	return nil
}

// Load reads a run manifest.
func (s *SQLiteStore) Load(id string) (domain.RunManifest, error) {
	var manifest domain.RunManifest
	err := s.db.QueryRow(`SELECT id, status, adapter, raw_demand FROM runs WHERE id = ?`, id).
		Scan(&manifest.ID, &manifest.Status, &manifest.Adapter, &manifest.RawDemand)
	if err != nil {
		return domain.RunManifest{}, fmt.Errorf("load run: %w", err)
	}
	return manifest, nil
}

// List returns all runs in insertion order by id for stable CLI output.
func (s *SQLiteStore) List() ([]domain.RunManifest, error) {
	rows, err := s.db.Query(`SELECT id, status, adapter, raw_demand FROM runs ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var manifests []domain.RunManifest
	for rows.Next() {
		var manifest domain.RunManifest
		if err := rows.Scan(&manifest.ID, &manifest.Status, &manifest.Adapter, &manifest.RawDemand); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		manifests = append(manifests, manifest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return manifests, nil
}

// Enqueue inserts a run into the single-job FIFO queue.
func (s *SQLiteStore) Enqueue(jobID string, runID string) error {
	if jobID == "" || runID == "" {
		return fmt.Errorf("job id and run id are required")
	}
	_, err := s.db.Exec(
		`INSERT INTO jobs (id, run_id, status) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET run_id=excluded.run_id, status=excluded.status`,
		jobID, runID, domain.JobQueued,
	)
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}
	return nil
}

// SetJobStatus updates a queued/running job state.
func (s *SQLiteStore) SetJobStatus(jobID string, status domain.JobStatus) error {
	if jobID == "" {
		return fmt.Errorf("job id is required")
	}
	_, err := s.db.Exec(`UPDATE jobs SET status = ? WHERE id = ?`, status, jobID)
	if err != nil {
		return fmt.Errorf("set job status: %w", err)
	}
	return nil
}

// NextQueued returns the oldest queued job.
func (s *SQLiteStore) NextQueued() (domain.JobRecord, error) {
	var job domain.JobRecord
	err := s.db.QueryRow(`SELECT id, run_id, status FROM jobs WHERE status = ? ORDER BY created_order LIMIT 1`, domain.JobQueued).
		Scan(&job.ID, &job.RunID, &job.Status)
	if err != nil {
		return domain.JobRecord{}, fmt.Errorf("next queued job: %w", err)
	}
	return job, nil
}

// SaveStep persists crash-recovery metadata for a step.
func (s *SQLiteStore) SaveStep(step domain.StepRecord) error {
	if step.RunID == "" || step.StepID == "" {
		return fmt.Errorf("run id and step id are required")
	}
	_, err := s.db.Exec(
		`INSERT INTO steps (run_id, step_id, status, commit_sha) VALUES (?, ?, ?, ?)
		 ON CONFLICT(run_id, step_id) DO UPDATE SET status=excluded.status, commit_sha=excluded.commit_sha`,
		step.RunID, step.StepID, step.Status, step.Commit,
	)
	if err != nil {
		return fmt.Errorf("save step: %w", err)
	}
	return nil
}

// ListSteps returns all step records for a run.
func (s *SQLiteStore) ListSteps(runID string) ([]domain.StepRecord, error) {
	rows, err := s.db.Query(`SELECT run_id, step_id, status, commit_sha FROM steps WHERE run_id = ? ORDER BY step_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	defer rows.Close()

	var steps []domain.StepRecord
	for rows.Next() {
		var step domain.StepRecord
		if err := rows.Scan(&step.RunID, &step.StepID, &step.Status, &step.Commit); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate steps: %w", err)
	}
	return steps, nil
}

// ReconcileCommittedSteps marks committing steps as committed when git history has the matching commit.
func (s *SQLiteStore) ReconcileCommittedSteps(runID string, commits []domain.CommitStep) error {
	for _, commit := range commits {
		if commit.StepID == "" || commit.Commit == "" {
			return fmt.Errorf("commit step requires step id and commit")
		}
		_, err := s.db.Exec(
			`UPDATE steps SET status = ? WHERE run_id = ? AND step_id = ? AND commit_sha = ?`,
			domain.StepCommitted, runID, commit.StepID, commit.Commit,
		)
		if err != nil {
			return fmt.Errorf("reconcile step %s: %w", commit.StepID, err)
		}
	}
	return nil
}
