package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	dfterrors "github.com/bocacorazon/dft/internal/errors"
	"github.com/bocacorazon/dft/internal/verify"
)

type RunRecord struct {
	RunID      string     `json:"run_id"`
	State      string     `json:"state"`
	FlowFile   string     `json:"flow_file"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type StepRecord struct {
	StepID    string `json:"step_id"`
	Capture   bool   `json:"capture"`
	ExportAs  string `json:"export_as,omitempty"`
	StdoutRef string `json:"stdout_ref"`
}

type Filesystem struct {
	root string
}

func NewFilesystem(root string) *Filesystem {
	return &Filesystem{root: root}
}

func (f *Filesystem) StartRun(runID string, flowFile string) error {
	if err := os.MkdirAll(f.runDir(runID), 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}

	record := RunRecord{
		RunID:     runID,
		State:     "running",
		FlowFile:  flowFile,
		StartedAt: time.Now().UTC(),
	}
	return f.writeRunRecord(record)
}

func (f *Filesystem) MarkRunRunning(runID string) error {
	record, err := f.GetRun(runID)
	if err != nil {
		return err
	}
	record.State = "running"
	record.FinishedAt = nil
	record.Error = ""
	return f.writeRunRecord(record)
}

func (f *Filesystem) MarkRunSucceeded(runID string) error {
	record, err := f.GetRun(runID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	record.State = "succeeded"
	record.FinishedAt = &now
	record.Error = ""
	return f.writeRunRecord(record)
}

func (f *Filesystem) MarkRunFailed(runID string, errMsg string) error {
	record, err := f.GetRun(runID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	record.State = "failed"
	record.FinishedAt = &now
	record.Error = errMsg
	return f.writeRunRecord(record)
}

func (f *Filesystem) GetRun(runID string) (RunRecord, error) {
	path := f.runMetaPath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RunRecord{}, dfterrors.RunNotFoundError{RunID: runID}
		}
		return RunRecord{}, fmt.Errorf("read run metadata: %w", err)
	}

	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return RunRecord{}, fmt.Errorf("decode run metadata: %w", err)
	}
	return record, nil
}

func (f *Filesystem) WriteStepOutput(runID string, stepID string, stdout string, capture bool, exportAs string) error {
	stepDir := filepath.Join(f.runDir(runID), stepID)
	if err := os.MkdirAll(stepDir, 0o755); err != nil {
		return fmt.Errorf("create step dir: %w", err)
	}

	stdoutPath := filepath.Join(stepDir, "stdout.txt")
	if err := os.WriteFile(stdoutPath, []byte(stdout), 0o644); err != nil {
		return fmt.Errorf("write stdout artifact: %w", err)
	}

	stepRecord := StepRecord{
		StepID:    stepID,
		Capture:   capture,
		ExportAs:  exportAs,
		StdoutRef: stdoutPath,
	}
	stepJSON, err := json.MarshalIndent(stepRecord, "", "  ")
	if err != nil {
		return fmt.Errorf("encode step metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stepDir, "step.json"), stepJSON, 0o644); err != nil {
		return fmt.Errorf("write step metadata: %w", err)
	}
	return nil
}

func (f *Filesystem) ReadStepStdout(runID string, stepID string) (string, error) {
	path := filepath.Join(f.runDir(runID), stepID, "stdout.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read step stdout: %w", err)
	}
	return string(data), nil
}

func (f *Filesystem) HasStepArtifact(runID string, stepID string) (bool, error) {
	path := filepath.Join(f.runDir(runID), stepID, "step.json")
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat step artifact: %w", err)
}

func (f *Filesystem) WriteVerifyFailures(runID string, failures []verify.Failure) error {
	if err := os.MkdirAll(f.runDir(runID), 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	data, err := json.MarshalIndent(failures, "", "  ")
	if err != nil {
		return fmt.Errorf("encode verify failures: %w", err)
	}
	path := filepath.Join(f.runDir(runID), "verify-failures.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write verify failures: %w", err)
	}
	return nil
}

func (f *Filesystem) runDir(runID string) string {
	return filepath.Join(f.root, runID)
}

func (f *Filesystem) runMetaPath(runID string) string {
	return filepath.Join(f.runDir(runID), "run.json")
}

func (f *Filesystem) writeRunRecord(record RunRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run metadata: %w", err)
	}
	if err := os.WriteFile(f.runMetaPath(record.RunID), data, 0o644); err != nil {
		return fmt.Errorf("write run metadata: %w", err)
	}
	return nil
}
