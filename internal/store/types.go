package store

import "github.com/bocacorazon/dft/internal/verify"

type RunStore interface {
	StartRun(runID string, flowFile string) error
	MarkRunRunning(runID string) error
	MarkRunSucceeded(runID string) error
	MarkRunFailed(runID string, errMsg string) error
	GetRun(runID string) (RunRecord, error)
	WriteStepOutput(runID string, stepID string, stdout string, capture bool, exportAs string) error
	ReadStepStdout(runID string, stepID string) (string, error)
	HasStepArtifact(runID string, stepID string) (bool, error)
	WriteVerifyFailures(runID string, failures []verify.Failure) error
}
