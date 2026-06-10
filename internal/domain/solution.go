package domain

import (
	"fmt"
	"time"
)

// TestScenario describes one behavior that must hold true.
type TestScenario struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	RequirementIDs []string `json:"requirement_ids"`
	Description    string   `json:"description"`
	Given          []string `json:"given"`
	When           []string `json:"when"`
	Then           []string `json:"then"`
}

// TestPlan is the full verification contract from the Solution phase.
type TestPlan struct {
	IncrementPackageID string         `json:"increment_package_id"`
	RequirementIDs     []string       `json:"requirement_ids"`
	Scenarios          []TestScenario `json:"scenarios"`
}

// SolutionDesign is the full contract output of the Solution phase.
type SolutionDesign struct {
	IncrementPackageID  string              `json:"increment_package_id"`
	TestPlan            TestPlan            `json:"test_plan"`
	WBS                 WBS                 `json:"wbs"`
	LaneAssignments     []LaneAssignment    `json:"lane_assignments"`
	EvalSurfaceContract EvalSurfaceContract `json:"eval_surface_contract"`
	CreatedAt           time.Time           `json:"created_at"`
}

// Validate returns an error when the solution design cannot drive orchestration.
func (sd SolutionDesign) Validate() error {
	if sd.IncrementPackageID == "" {
		return fmt.Errorf("increment package id is required")
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
