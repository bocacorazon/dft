package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	dfterrors "github.com/bocacorazon/dft/internal/errors"
	"github.com/bocacorazon/dft/internal/verify"
)

type SQLite struct {
	db        *sql.DB
	artifacts *Filesystem
}

func NewSQLite(dbPath string, artifactsRoot string) (*SQLite, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	schema := `
CREATE TABLE IF NOT EXISTS runs (
  run_id TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  flow_file TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  error TEXT
);`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &SQLite{
		db:        db,
		artifacts: NewFilesystem(artifactsRoot),
	}, nil
}

func (s *SQLite) StartRun(runID string, flowFile string) error {
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(
		`INSERT INTO runs(run_id, state, flow_file, started_at) VALUES (?, ?, ?, ?)`,
		runID, "running", flowFile, startedAt,
	); err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return s.artifacts.StartRun(runID, flowFile)
}

func (s *SQLite) MarkRunRunning(runID string) error {
	if _, err := s.db.Exec(
		`UPDATE runs SET state = ?, finished_at = NULL, error = '' WHERE run_id = ?`,
		"running", runID,
	); err != nil {
		return fmt.Errorf("update run running: %w", err)
	}
	return s.artifacts.MarkRunRunning(runID)
}

func (s *SQLite) MarkRunSucceeded(runID string) error {
	finishedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(
		`UPDATE runs SET state = ?, finished_at = ?, error = '' WHERE run_id = ?`,
		"succeeded", finishedAt, runID,
	); err != nil {
		return fmt.Errorf("update run success: %w", err)
	}
	return s.artifacts.MarkRunSucceeded(runID)
}

func (s *SQLite) MarkRunFailed(runID string, errMsg string) error {
	finishedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(
		`UPDATE runs SET state = ?, finished_at = ?, error = ? WHERE run_id = ?`,
		"failed", finishedAt, errMsg, runID,
	); err != nil {
		return fmt.Errorf("update run failure: %w", err)
	}
	return s.artifacts.MarkRunFailed(runID, errMsg)
}

func (s *SQLite) GetRun(runID string) (RunRecord, error) {
	row := s.db.QueryRow(`SELECT run_id, state, flow_file, started_at, finished_at, error FROM runs WHERE run_id = ?`, runID)
	var record RunRecord
	var startedAt string
	var finishedAt sql.NullString
	var errText sql.NullString
	if err := row.Scan(&record.RunID, &record.State, &record.FlowFile, &startedAt, &finishedAt, &errText); err != nil {
		if err == sql.ErrNoRows {
			return RunRecord{}, dfterrors.RunNotFoundError{RunID: runID}
		}
		return RunRecord{}, fmt.Errorf("query run: %w", err)
	}
	start, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return RunRecord{}, fmt.Errorf("parse started_at: %w", err)
	}
	record.StartedAt = start
	if finishedAt.Valid {
		finish, err := time.Parse(time.RFC3339Nano, finishedAt.String)
		if err != nil {
			return RunRecord{}, fmt.Errorf("parse finished_at: %w", err)
		}
		record.FinishedAt = &finish
	}
	if errText.Valid {
		record.Error = errText.String
	}
	return record, nil
}

func (s *SQLite) WriteStepOutput(runID string, stepID string, stdout string, capture bool, exportAs string) error {
	return s.artifacts.WriteStepOutput(runID, stepID, stdout, capture, exportAs)
}

func (s *SQLite) ReadStepStdout(runID string, stepID string) (string, error) {
	return s.artifacts.ReadStepStdout(runID, stepID)
}

func (s *SQLite) HasStepArtifact(runID string, stepID string) (bool, error) {
	return s.artifacts.HasStepArtifact(runID, stepID)
}

func (s *SQLite) WriteVerifyFailures(runID string, failures []verify.Failure) error {
	return s.artifacts.WriteVerifyFailures(runID, failures)
}
