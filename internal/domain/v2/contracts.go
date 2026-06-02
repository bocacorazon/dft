package v2

import "fmt"

// Validate returns an error when the demand package cannot guide solution design.
func (dp DemandPackage) Validate() error {
	if dp.ID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if dp.Title == "" {
		return fmt.Errorf("demand package title is required")
	}
	if dp.RawDemand == "" {
		return fmt.Errorf("raw demand is required")
	}
	if dp.RefinedDemand == "" {
		return fmt.Errorf("refined demand is required")
	}
	if len(dp.AcceptanceCriteria) == 0 {
		return fmt.Errorf("at least one acceptance criterion is required")
	}
	for _, ac := range dp.AcceptanceCriteria {
		if ac.ID == "" {
			return fmt.Errorf("acceptance criterion id is required")
		}
		if ac.Description == "" {
			return fmt.Errorf("acceptance criterion %q description is required", ac.ID)
		}
	}
	if !dp.VerifiedComplete {
		return fmt.Errorf("acceptance criteria not verified complete")
	}
	return nil
}

// Validate returns an error when the solution design cannot drive orchestration.
func (sd SolutionDesign) Validate() error {
	if sd.DemandPackageID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if len(sd.TestPlan.Scenarios) == 0 {
		return fmt.Errorf("at least one test scenario is required")
	}
	for _, s := range sd.TestPlan.Scenarios {
		if s.ID == "" {
			return fmt.Errorf("test scenario id is required")
		}
		if s.Name == "" {
			return fmt.Errorf("test scenario %q name is required", s.ID)
		}
		if len(s.Then) == 0 {
			return fmt.Errorf("test scenario %q requires at least one 'then' clause", s.ID)
		}
	}
	if err := sd.WBS.Validate(); err != nil {
		return fmt.Errorf("wbs: %w", err)
	}
	if err := sd.EvalSurfaceContract.Validate(); err != nil {
		return fmt.Errorf("eval surface contract: %w", err)
	}
	return nil
}

// Validate returns an error when the orchestration result is incomplete.
func (or OrchestrationResult) Validate() error {
	if or.DemandPackageID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if or.IncrementBranch == "" {
		return fmt.Errorf("increment branch is required")
	}
	return nil
}