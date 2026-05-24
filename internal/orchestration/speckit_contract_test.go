package orchestration

import (
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
)

func TestSpecKitStageContractsDescribeCurrentLane(t *testing.T) {
	definition := BuildBaseSpecKitFlow(domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
	})
	contracts := SpecKitStageContracts()
	if len(contracts) != 8 {
		t.Fatalf("contract count = %d, want 8", len(contracts))
	}

	wantOrder := []SpecKitStageID{
		SpecKitStageSpecify,
		SpecKitStagePlan,
		SpecKitStageTasks,
		SpecKitStageAnalyze,
		SpecKitStageImplement,
		SpecKitStageCodeReview,
		SpecKitStageIssueHandoff,
		SpecKitStageMergeback,
	}
	seenExecutionIDs := map[string]struct{}{}
	for i, contract := range contracts {
		if contract.ID != wantOrder[i] {
			t.Fatalf("contract[%d] id = %q, want %q", i, contract.ID, wantOrder[i])
		}
		if contract.BlockingPolicy == "" {
			t.Fatalf("contract %q blocking policy is empty", contract.ID)
		}
		if contract.PrimaryStepID == "" {
			t.Fatalf("contract %q primary step id is empty", contract.ID)
		}
		if _, ok := findStep(definition.Steps, contract.PrimaryStepID); !ok {
			t.Fatalf("contract %q primary step %q missing from lane", contract.ID, contract.PrimaryStepID)
		}
		for _, stepID := range contract.ControlStepIDs {
			if _, ok := findStep(definition.Steps, stepID); !ok {
				t.Fatalf("contract %q control step %q missing from lane", contract.ID, stepID)
			}
		}
		for _, checkID := range contract.VerificationCheckIDs {
			if !definitionHasCheckID(definition, checkID) {
				t.Fatalf("contract %q verification check %q missing from lane", contract.ID, checkID)
			}
		}
		executionID := contract.ExecutionID("001-real-submit", 1)
		if executionID == "" {
			t.Fatalf("contract %q returned empty execution id", contract.ID)
		}
		if _, ok := seenExecutionIDs[executionID]; ok {
			t.Fatalf("duplicate execution id %q", executionID)
		}
		seenExecutionIDs[executionID] = struct{}{}
	}
}

func TestSpecKitStageContractDetails(t *testing.T) {
	plan, ok := SpecKitStageContractByID(SpecKitStagePlan)
	if !ok {
		t.Fatal("plan contract not found")
	}
	if plan.BlockingPolicy != SpecKitBlockingRetry {
		t.Fatalf("plan blocking policy = %q, want %q", plan.BlockingPolicy, SpecKitBlockingRetry)
	}
	if len(plan.OptionalArtifacts) == 0 {
		t.Fatal("plan optional artifacts empty, want conditional plan outputs")
	}

	tasks, ok := SpecKitStageContractByID(SpecKitStageTasks)
	if !ok {
		t.Fatal("tasks contract not found")
	}
	if tasks.FindingsStepID != "analyze" {
		t.Fatalf("tasks findings step = %q, want analyze", tasks.FindingsStepID)
	}
	if tasks.BlockingPolicy != SpecKitBlockingRemediate {
		t.Fatalf("tasks blocking policy = %q, want %q", tasks.BlockingPolicy, SpecKitBlockingRemediate)
	}

	codeReview, ok := SpecKitStageContractByID(SpecKitStageCodeReview)
	if !ok {
		t.Fatal("code-review contract not found")
	}
	if codeReview.FindingsStepID != "code-review" {
		t.Fatalf("code-review findings step = %q, want code-review", codeReview.FindingsStepID)
	}

	mergeback, ok := SpecKitStageContractByID(SpecKitStageMergeback)
	if !ok {
		t.Fatal("mergeback contract not found")
	}
	if mergeback.BlockingPolicy != SpecKitBlockingResolveConflicts {
		t.Fatalf("mergeback blocking policy = %q, want %q", mergeback.BlockingPolicy, SpecKitBlockingResolveConflicts)
	}
	if mergeback.ExecutionID("001-real-submit", 2) != "speckit/001-real-submit/mergeback/attempt-02" {
		t.Fatalf("mergeback execution id = %q", mergeback.ExecutionID("001-real-submit", 2))
	}
}

func definitionHasCheckID(definition flow.Definition, checkID string) bool {
	for _, step := range definition.Steps {
		if stepHasCheckID(step, checkID) {
			return true
		}
	}
	return false
}

func stepHasCheckID(step flow.Step, checkID string) bool {
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
		if stepHasCheckID(nested, checkID) {
			return true
		}
	}
	for _, nested := range step.Steps {
		if stepHasCheckID(nested, checkID) {
			return true
		}
	}
	return false
}
