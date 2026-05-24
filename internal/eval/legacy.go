package eval

import "github.com/bocacorazon/dft/internal/domain"

// FromLegacyPlan preserves existing deterministic checks as a first-class eval plan.
func FromLegacyPlan(demandPackageID string, plan domain.EvaluationPlan) domain.EvalPlan {
	return domain.EvalPlan{
		DemandPackageID: demandPackageID,
		Checks:          append([]domain.Check(nil), plan.Checks...),
	}
}

// ToVerificationResult adapts the new eval result to the existing remediation/merge gate type.
func ToVerificationResult(result domain.EvalResult) domain.VerificationResult {
	status := domain.VerdictPass
	if result.Status != domain.EvalStatusPass {
		status = domain.VerdictFail
	}
	return domain.VerificationResult{
		Status:   status,
		Results:  append([]domain.CheckResult(nil), result.Checks...),
		Findings: append([]domain.Finding(nil), result.Findings...),
	}
}
