package eval

import (
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestFromLegacyPlanPreservesDeterministicChecks(t *testing.T) {
	plan := FromLegacyPlan("demand-1", domain.EvaluationPlan{Checks: []domain.Check{{
		ID:   "done",
		Kind: domain.CheckFileExists,
		Args: []string{"done.txt"},
	}}})

	if plan.DemandPackageID != "demand-1" {
		t.Fatalf("DemandPackageID = %q, want demand-1", plan.DemandPackageID)
	}
	if len(plan.Checks) != 1 || plan.Checks[0].ID != "done" {
		t.Fatalf("checks = %#v, want legacy check", plan.Checks)
	}
}

func TestToVerificationResultMapsBlockedEvalToFailedVerdict(t *testing.T) {
	result := ToVerificationResult(domain.EvalResult{
		Status: domain.EvalStatusBlocked,
		Findings: []domain.Finding{{
			CheckID: "surface",
			Message: "missing surface",
		}},
	})

	if result.Status != domain.VerdictFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(result.Findings))
	}
}
