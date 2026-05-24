package domain

import "fmt"

// VerdictStatus is the aggregate verification outcome.
type VerdictStatus string

const (
	VerdictPass VerdictStatus = "pass"
	VerdictFail VerdictStatus = "fail"
)

// CheckKind names a deterministic verification predicate.
type CheckKind string

const (
	CheckFileExists          CheckKind = "file_exists"
	CheckFileMissing         CheckKind = "file_missing"
	CheckCommandExitZero     CheckKind = "command_exit_zero"
	CheckGrepMatches         CheckKind = "grep_matches"
	CheckJSONPathEquals      CheckKind = "json_path_equals"
	CheckCountMatchesAtLeast CheckKind = "count_matches_at_least"
	CheckCountMatchesEquals  CheckKind = "count_matches_equals"
	CheckFileChecksumDiffers CheckKind = "file_checksum_differs"
	CheckGitNoUnmergedFiles  CheckKind = "git_no_unmerged_files"
	CheckStringEquals        CheckKind = "string_equals"
	CheckOS                  CheckKind = "os"
	CheckNoBinaryArtifacts   CheckKind = "no_binary_artifacts"
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
	CheckID   string `json:"check_id,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Message   string `json:"message"`
	Category  string `json:"category,omitempty"`
	Location  string `json:"location,omitempty"`
	Advice    string `json:"advice,omitempty"`
	FindingID string `json:"finding_id,omitempty"`
}

// VerificationResult is the aggregate deterministic check result.
type VerificationResult struct {
	Status   VerdictStatus `json:"status"`
	Results  []CheckResult `json:"results"`
	Findings []Finding     `json:"findings,omitempty"`
}

// EvaluationPlan is the deterministic check plan authored after build.
type EvaluationPlan struct {
	Checks []Check `json:"checks"`
}

// Validate returns an error when the plan cannot be executed safely.
func (p EvaluationPlan) Validate() error {
	if len(p.Checks) == 0 {
		return fmt.Errorf("at least one evaluation check is required")
	}
	for _, check := range p.Checks {
		if check.ID == "" {
			return fmt.Errorf("evaluation check id is required")
		}
		if check.Kind == "" {
			return fmt.Errorf("evaluation check %q kind is required", check.ID)
		}
	}
	return nil
}
