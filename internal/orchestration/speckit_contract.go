package orchestration

import (
	"fmt"
	"strings"

	"github.com/bocacorazon/dft/internal/flow"
)

// SpecKitStageID names one user-visible stage in the single-spec Speckit lane.
type SpecKitStageID string

const (
	SpecKitStageSpecify      SpecKitStageID = "specify"
	SpecKitStagePlan         SpecKitStageID = "plan"
	SpecKitStageTasks        SpecKitStageID = "tasks"
	SpecKitStageAnalyze      SpecKitStageID = "analyze"
	SpecKitStageImplement    SpecKitStageID = "implement"
	SpecKitStageCodeReview   SpecKitStageID = "code-review"
	SpecKitStageIssueHandoff SpecKitStageID = "issue-handoff"
	SpecKitStageMergeback    SpecKitStageID = "mergeback"
)

// SpecKitBlockingPolicy captures the expected operator-facing stop behavior.
type SpecKitBlockingPolicy string

const (
	SpecKitBlockingRetry            SpecKitBlockingPolicy = "retry"
	SpecKitBlockingRemediate        SpecKitBlockingPolicy = "remediate"
	SpecKitBlockingResolveConflicts SpecKitBlockingPolicy = "resolve_conflicts"
	SpecKitBlockingContinue         SpecKitBlockingPolicy = "continue"
)

// SpecKitStageStatus is the canonical lifecycle state for one stage attempt.
type SpecKitStageStatus string

const (
	SpecKitStagePending   SpecKitStageStatus = "pending"
	SpecKitStageRunning   SpecKitStageStatus = "running"
	SpecKitStageBlocked   SpecKitStageStatus = "blocked"
	SpecKitStageFailed    SpecKitStageStatus = "failed"
	SpecKitStageSucceeded SpecKitStageStatus = "succeeded"
)

// SpecKitStageArtifact describes a stage-owned artifact contract.
type SpecKitStageArtifact struct {
	Name           string
	PathExpression string
	Required       bool
}

// SpecKitStageContract is the source-of-truth contract for one lane stage.
type SpecKitStageContract struct {
	ID                   SpecKitStageID
	PrimaryStepID        string
	ResumeStepID         string
	ControlStepIDs       []string
	VerificationCheckIDs []string
	RequiredArtifacts    []SpecKitStageArtifact
	OptionalArtifacts    []SpecKitStageArtifact
	BlockingPolicy       SpecKitBlockingPolicy
	FindingsStepID       string
	Lifecycle            []SpecKitStageStatus
}

var specKitStageContracts = []SpecKitStageContract{
	{
		ID:                   SpecKitStageSpecify,
		PrimaryStepID:        "specify",
		ResumeStepID:         "specify",
		VerificationCheckIDs: []string{"spec-file", "spec-differs-template", "spec-requirements"},
		RequiredArtifacts: []SpecKitStageArtifact{
			{Name: "spec", PathExpression: "{{ steps.specify.output.artifacts.spec_file }}", Required: true},
			{Name: "requirements", PathExpression: "{{ steps.specify.output.artifacts.requirements_file }}", Required: true},
		},
		BlockingPolicy: SpecKitBlockingRetry,
		Lifecycle:      defaultSpecKitStageLifecycle(),
	},
	{
		ID:                   SpecKitStagePlan,
		PrimaryStepID:        "plan",
		ResumeStepID:         "plan",
		VerificationCheckIDs: []string{"plan-file", "plan-differs-template", "research-file"},
		RequiredArtifacts: []SpecKitStageArtifact{
			{Name: "plan", PathExpression: "{{ steps.plan.output.artifacts.plan_file }}", Required: true},
			{Name: "research", PathExpression: "{{ steps.plan.output.artifacts.research_file }}", Required: true},
		},
		OptionalArtifacts: []SpecKitStageArtifact{
			{Name: "contracts", PathExpression: "{{ vars.feature_directory }}/contracts", Required: false},
			{Name: "data-model", PathExpression: "{{ vars.feature_directory }}/data-model.md", Required: false},
			{Name: "quickstart", PathExpression: "{{ vars.feature_directory }}/quickstart.md", Required: false},
		},
		BlockingPolicy: SpecKitBlockingRetry,
		Lifecycle:      defaultSpecKitStageLifecycle(),
	},
	{
		ID:                   SpecKitStageTasks,
		PrimaryStepID:        "tasks",
		ResumeStepID:         "tasks",
		ControlStepIDs:       []string{"tasks-remediation"},
		VerificationCheckIDs: []string{"tasks-file", "tasks-differs-template", "tasks-contains-task-lines"},
		RequiredArtifacts: []SpecKitStageArtifact{
			{Name: "tasks", PathExpression: "{{ steps.tasks.output.artifacts.tasks_file }}", Required: true},
		},
		BlockingPolicy: SpecKitBlockingRemediate,
		FindingsStepID: "analyze",
		Lifecycle:      defaultSpecKitStageLifecycle(),
	},
	{
		ID:                   SpecKitStageAnalyze,
		PrimaryStepID:        "analyze",
		ResumeStepID:         "analyze",
		ControlStepIDs:       []string{"analyze-clean", "tasks-remediation"},
		VerificationCheckIDs: []string{"no-blocking-analysis-findings"},
		BlockingPolicy:       SpecKitBlockingRemediate,
		FindingsStepID:       "analyze",
		Lifecycle:            defaultSpecKitStageLifecycle(),
	},
	{
		ID:                   SpecKitStageImplement,
		PrimaryStepID:        "implement",
		ResumeStepID:         "implement-review-loop",
		ControlStepIDs:       []string{"implement-review-loop"},
		VerificationCheckIDs: []string{"implement-task-progress"},
		RequiredArtifacts: []SpecKitStageArtifact{
			{Name: "tasks", PathExpression: "{{ steps.implement.output.artifacts.tasks_file }}", Required: true},
		},
		BlockingPolicy: SpecKitBlockingRemediate,
		Lifecycle:      defaultSpecKitStageLifecycle(),
	},
	{
		ID:                   SpecKitStageCodeReview,
		PrimaryStepID:        "code-review",
		ResumeStepID:         "implement-review-loop",
		ControlStepIDs:       []string{"implement-review-loop", "review-clean"},
		VerificationCheckIDs: []string{"review-no-critical-findings"},
		BlockingPolicy:       SpecKitBlockingRemediate,
		FindingsStepID:       "code-review",
		Lifecycle:            defaultSpecKitStageLifecycle(),
	},
	{
		ID:             SpecKitStageIssueHandoff,
		PrimaryStepID:  "issues-from-review",
		ResumeStepID:   "issues-from-review",
		BlockingPolicy: SpecKitBlockingContinue,
		Lifecycle:      defaultSpecKitStageLifecycle(),
	},
	{
		ID:                   SpecKitStageMergeback,
		PrimaryStepID:        "mergeback-attempt",
		ResumeStepID:         "commit-before-mergeback",
		ControlStepIDs:       []string{"commit-before-mergeback", "resolve-mergeback", "mergeback-finalize", "verify-mergeback"},
		VerificationCheckIDs: []string{"mergeback-no-conflicts", "mergeback-trees-equal", "mergeback-local-branch-deleted", "mergeback-remote-branch-deleted"},
		BlockingPolicy:       SpecKitBlockingResolveConflicts,
		Lifecycle:            defaultSpecKitStageLifecycle(),
	},
}

func defaultSpecKitStageLifecycle() []SpecKitStageStatus {
	return []SpecKitStageStatus{
		SpecKitStagePending,
		SpecKitStageRunning,
		SpecKitStageBlocked,
		SpecKitStageFailed,
		SpecKitStageSucceeded,
	}
}

// SpecKitStageContracts returns the ordered single-spec Speckit lane stages.
func SpecKitStageContracts() []SpecKitStageContract {
	contracts := make([]SpecKitStageContract, len(specKitStageContracts))
	copy(contracts, specKitStageContracts)
	return contracts
}

// SpecKitStageContractByID returns the contract for one stage.
func SpecKitStageContractByID(id SpecKitStageID) (SpecKitStageContract, bool) {
	for _, contract := range specKitStageContracts {
		if contract.ID == id {
			return contract, true
		}
	}
	return SpecKitStageContract{}, false
}

// ExecutionID returns a stable identifier for one stage attempt.
func (c SpecKitStageContract) ExecutionID(specID string, attempt int) string {
	if attempt < 1 {
		attempt = 1
	}
	return fmt.Sprintf("speckit/%s/%s/attempt-%02d", strings.TrimSpace(specID), c.ID, attempt)
}

func validateSpecKitLaneDefinition(definition flow.Definition) error {
	for _, contract := range SpecKitStageContracts() {
		if contract.PrimaryStepID == "" {
			return fmt.Errorf("stage contract %q primary step id is required", contract.ID)
		}
		if contract.ResumeStepID == "" {
			return fmt.Errorf("stage contract %q resume step id is required", contract.ID)
		}
		if _, ok := lookupSpecKitStep(definition.Steps, contract.PrimaryStepID); !ok {
			return fmt.Errorf("stage contract %q primary step %q missing from lane", contract.ID, contract.PrimaryStepID)
		}
		if _, ok := lookupSpecKitStep(definition.Steps, contract.ResumeStepID); !ok {
			return fmt.Errorf("stage contract %q resume step %q missing from lane", contract.ID, contract.ResumeStepID)
		}
		for _, stepID := range contract.ControlStepIDs {
			if _, ok := lookupSpecKitStep(definition.Steps, stepID); !ok {
				return fmt.Errorf("stage contract %q control step %q missing from lane", contract.ID, stepID)
			}
		}
		for _, checkID := range contract.VerificationCheckIDs {
			if !definitionContainsCheckID(definition.Steps, checkID) {
				return fmt.Errorf("stage contract %q verification check %q missing from lane", contract.ID, checkID)
			}
		}
	}
	return nil
}

func lookupSpecKitStep(steps []flow.Step, id string) (flow.Step, bool) {
	for _, step := range steps {
		if step.ID == id {
			return step, true
		}
		if nested, ok := lookupSpecKitStep(step.Setup, id); ok {
			return nested, true
		}
		if nested, ok := lookupSpecKitStep(step.Steps, id); ok {
			return nested, true
		}
	}
	return flow.Step{}, false
}

func definitionContainsCheckID(steps []flow.Step, checkID string) bool {
	for _, step := range steps {
		if stepContainsCheckID(step, checkID) {
			return true
		}
	}
	return false
}

func stepContainsCheckID(step flow.Step, checkID string) bool {
	for _, check := range step.Verify {
		if check.ID == checkID {
			return true
		}
	}
	for _, check := range step.Checks {
		if check.ID == checkID {
			return true
		}
	}
	for _, nested := range step.Setup {
		if stepContainsCheckID(nested, checkID) {
			return true
		}
	}
	for _, nested := range step.Steps {
		if stepContainsCheckID(nested, checkID) {
			return true
		}
	}
	return false
}
