package domain

// JobStatus is the durable state of queued increment-package work.
type JobStatus string

const (
	JobQueued  JobStatus = "queued"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

// JobRecord stores FIFO queue membership for a run.
type JobRecord struct {
	ID     string    `json:"id"`
	RunID  string    `json:"run_id"`
	Status JobStatus `json:"status"`
}

// InboxEntry is a durable human-facing escalation or manual gate.
type InboxEntry struct {
	ID      string `json:"id"`
	RunID   string `json:"run_id"`
	StepID  string `json:"step_id,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
