package domain

import "fmt"

// SpecRef identifies one independently executable spec in a WBS.
type SpecRef struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description"`
	PromptPath         string   `json:"prompt_path,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	Lane               string   `json:"lane,omitempty"`
	Workflow           string   `json:"workflow,omitempty"`
	WorkflowPath       string   `json:"workflow_path,omitempty"`
	FeatureSlug        string   `json:"feature_slug,omitempty"`
	ModelFamily        string   `json:"model_family,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// WBS is the append-only work breakdown structure for an increment package.
type WBS struct {
	IncrementPackageID string    `json:"increment_package_id"`
	BaseBranch         string    `json:"base_branch,omitempty"`
	IncrementBranch    string    `json:"increment_branch,omitempty"`
	Specs              []SpecRef `json:"specs"`
}

// Validate returns an error when the WBS cannot drive orchestration.
func (w WBS) Validate() error {
	if w.IncrementPackageID == "" {
		return fmt.Errorf("increment package id is required")
	}
	if w.BaseBranch != "" {
		if err := ValidateBranchRef("base branch", w.BaseBranch); err != nil {
			return err
		}
	}
	if w.IncrementBranch != "" {
		if err := ValidateBranchRef("increment branch", w.IncrementBranch); err != nil {
			return err
		}
	}
	if len(w.Specs) == 0 {
		return fmt.Errorf("at least one spec is required")
	}
	specIDs := make(map[string]struct{}, len(w.Specs))
	for _, spec := range w.Specs {
		if spec.ID == "" {
			return fmt.Errorf("spec id is required")
		}
		if _, exists := specIDs[spec.ID]; exists {
			return fmt.Errorf("duplicate spec id %q", spec.ID)
		}
		specIDs[spec.ID] = struct{}{}
		if spec.Description == "" && spec.PromptPath == "" {
			return fmt.Errorf("spec %q description or prompt_path is required", spec.ID)
		}
		if len(spec.AcceptanceCriteria) == 0 {
			return fmt.Errorf("spec %q acceptance criteria are required", spec.ID)
		}
		if spec.Workflow != "" && spec.WorkflowPath != "" {
			return fmt.Errorf("spec %q must not set both workflow and workflow_path", spec.ID)
		}
		for _, dep := range spec.DependsOn {
			if dep == spec.ID {
				return fmt.Errorf("spec %q cannot depend on itself", spec.ID)
			}
		}
	}
	for _, spec := range w.Specs {
		for _, dep := range spec.DependsOn {
			if _, ok := specIDs[dep]; !ok {
				return fmt.Errorf("spec %q depends on unknown spec %q", spec.ID, dep)
			}
		}
	}
	return nil
}

// WBSAmendment captures Fix-Planner remediation after failed evaluation.
type WBSAmendment struct {
	IncrementPackageID     string             `json:"increment_package_id"`
	Findings               []Finding          `json:"findings"`
	RemediationSpecs       []SpecRef          `json:"remediation_specs,omitempty"`
	ChildIncrementPackages []IncrementPackage `json:"child_increment_packages,omitempty"`
}

// Validate returns an error when no actionable remediation was produced.
func (a WBSAmendment) Validate() error {
	if a.IncrementPackageID == "" {
		return fmt.Errorf("increment package id is required")
	}
	if len(a.Findings) == 0 {
		return fmt.Errorf("at least one finding is required")
	}
	if len(a.RemediationSpecs) == 0 && len(a.ChildIncrementPackages) == 0 {
		return fmt.Errorf("at least one remediation spec or child increment package is required")
	}
	for _, spec := range a.RemediationSpecs {
		if spec.ID == "" {
			return fmt.Errorf("remediation spec id is required")
		}
		if spec.Description == "" && spec.PromptPath == "" {
			return fmt.Errorf("remediation spec %q description or prompt_path is required", spec.ID)
		}
		if len(spec.AcceptanceCriteria) == 0 {
			return fmt.Errorf("remediation spec %q acceptance criteria are required", spec.ID)
		}
	}
	for _, child := range a.ChildIncrementPackages {
		if err := child.Validate(); err != nil {
			return fmt.Errorf("validate child increment package: %w", err)
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
