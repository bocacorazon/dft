package domain

// JobStatus is the durable state of queued demand-package work.
type JobStatus string

const (
	JobQueued  JobStatus = "queued"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
)

// JobRecord stores FIFO queue membership for a run.
type JobRecord struct {
	ID     string    `json:"id"`
	RunID  string    `json:"run_id"`
	Status JobStatus `json:"status"`
}

// DurableStepStatus is the persisted lifecycle state for one local-mutating step.
type DurableStepStatus string

const (
	StepPending    DurableStepStatus = "pending"
	StepCommitting DurableStepStatus = "committing"
	StepCommitted  DurableStepStatus = "committed"
)

// StepRecord stores crash-recovery metadata for a completed or in-flight step.
type StepRecord struct {
	RunID  string            `json:"run_id"`
	StepID string            `json:"step_id"`
	Status DurableStepStatus `json:"status"`
	Commit string            `json:"commit,omitempty"`
}

// CommitStep is discovered from git history during crash reconciliation.
type CommitStep struct {
	StepID string
	Commit string
}

// InboxEntry is a durable human-facing escalation or manual gate.
type InboxEntry struct {
	ID      string `json:"id"`
	RunID   string `json:"run_id"`
	StepID  string `json:"step_id,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
