package domain

import (
	"fmt"
	"time"
)

// AcceptanceCriterion is one atomic, testable requirement.
type AcceptanceCriterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Requirement string `json:"requirement,omitempty"` // links to REQ-xxx
}

// AmbiguityFinding records something unclear in the raw increment.
type AmbiguityFinding struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Resolution  string `json:"resolution,omitempty"` // filled after refinement
}

// IncrementPackage is the normalized unit of progress accepted by dft and the
// contract output of the Intent phase.
type IncrementPackage struct {
	ID                 string                `json:"id"`
	Title              string                `json:"title"`
	RawIncrement       string                `json:"raw_increment"`
	RefinedIncrement   string                `json:"refined_increment"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
	Assumptions        []string              `json:"assumptions,omitempty"`
	NonGoals           []string              `json:"non_goals,omitempty"`
	Ambiguities        []AmbiguityFinding    `json:"ambiguities,omitempty"`
	VerifiedComplete   bool                  `json:"verified_complete"`
	CreatedAt          time.Time             `json:"created_at"`
}

// Validate returns an error when the increment package cannot guide solution design.
func (dp IncrementPackage) Validate() error {
	if dp.ID == "" {
		return fmt.Errorf("increment package id is required")
	}
	if dp.Title == "" {
		return fmt.Errorf("increment package title is required")
	}
	if dp.RawIncrement == "" {
		return fmt.Errorf("raw increment is required")
	}
	if dp.RefinedIncrement == "" {
		return fmt.Errorf("refined increment is required")
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
