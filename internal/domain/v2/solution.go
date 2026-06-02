package v2

import (
	"time"

	"github.com/bocacorazon/dft/internal/domain"
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
	DemandPackageID string         `json:"demand_package_id"`
	RequirementIDs  []string       `json:"requirement_ids"`
	Scenarios       []TestScenario `json:"scenarios"`
}

// SolutionDesign is the full contract output of the Solution phase.
// WBS and LaneAssignments use the existing domain schemas from wbs.go.
type SolutionDesign struct {
	DemandPackageID     string                      `json:"demand_package_id"`
	TestPlan            TestPlan                    `json:"test_plan"`
	WBS                 domain.WBS                  `json:"wbs"`
	LaneAssignments     []domain.LaneAssignment     `json:"lane_assignments"`
	EvalSurfaceContract domain.EvalSurfaceContract  `json:"eval_surface_contract"`
	CreatedAt           time.Time                   `json:"created_at"`
}