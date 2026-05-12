package domain

// ReviewDecision records the final human or agent review gate.
type ReviewDecision struct {
	Approved bool      `json:"approved"`
	Findings []Finding `json:"findings,omitempty"`
}
