package domain

// RunStatus is the durable lifecycle state for a dft run.
type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

// RunManifest is the durable summary written for every run.
type RunManifest struct {
	ID        string    `json:"id"`
	Status    RunStatus `json:"status"`
	Adapter   string    `json:"adapter"`
	RawDemand string    `json:"raw_demand"`
}
