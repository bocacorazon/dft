package domain

// VerdictStatus is the aggregate verification outcome.
type VerdictStatus string

const (
	VerdictPass VerdictStatus = "pass"
	VerdictFail VerdictStatus = "fail"
)

// CheckKind names a deterministic verification predicate.
type CheckKind string

const (
	CheckFileExists      CheckKind = "file_exists"
	CheckFileMissing     CheckKind = "file_missing"
	CheckCommandExitZero CheckKind = "command_exit_zero"
	CheckGrepMatches     CheckKind = "grep_matches"
	CheckJSONPathEquals  CheckKind = "json_path_equals"
)

// Check is one deterministic verification predicate.
type Check struct {
	ID   string    `json:"id"`
	Kind CheckKind `json:"kind"`
	Args []string  `json:"args"`
}

// CheckResult captures one check outcome.
type CheckResult struct {
	CheckID string `json:"check_id"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// Finding is an actionable failure discovered by verification or review.
type Finding struct {
	CheckID string `json:"check_id,omitempty"`
	Message string `json:"message"`
}

// VerificationResult is the aggregate deterministic check result.
type VerificationResult struct {
	Status   VerdictStatus `json:"status"`
	Results  []CheckResult `json:"results"`
	Findings []Finding     `json:"findings,omitempty"`
}
