package domain

import "fmt"

// SpecRef identifies one independently executable spec in a WBS.
type SpecRef struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// WBS is the append-only work breakdown structure for a demand package.
type WBS struct {
	DemandPackageID string    `json:"demand_package_id"`
	Specs           []SpecRef `json:"specs"`
}

// Validate returns an error when the WBS cannot drive orchestration.
func (w WBS) Validate() error {
	if w.DemandPackageID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if len(w.Specs) == 0 {
		return fmt.Errorf("at least one spec is required")
	}
	for _, spec := range w.Specs {
		if spec.ID == "" {
			return fmt.Errorf("spec id is required")
		}
		if spec.Description == "" {
			return fmt.Errorf("spec %q description is required", spec.ID)
		}
		if len(spec.AcceptanceCriteria) == 0 {
			return fmt.Errorf("spec %q acceptance criteria are required", spec.ID)
		}
	}
	return nil
}

// LaneAssignment binds a spec to a lane selected for execution.
type LaneAssignment struct {
	SpecID    string `json:"spec_id"`
	Lane      string `json:"lane"`
	Rationale string `json:"rationale"`
}

// ValidateLaneAssignments verifies every assignment is actionable.
func ValidateLaneAssignments(assignments []LaneAssignment) error {
	if len(assignments) == 0 {
		return fmt.Errorf("at least one lane assignment is required")
	}
	for _, assignment := range assignments {
		if assignment.SpecID == "" {
			return fmt.Errorf("lane assignment spec id is required")
		}
		if assignment.Lane == "" {
			return fmt.Errorf("lane assignment for %q requires a lane", assignment.SpecID)
		}
	}
	return nil
}
