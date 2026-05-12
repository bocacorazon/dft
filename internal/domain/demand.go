package domain

import "fmt"

// DemandPackage is the normalized unit of progress accepted by dft.
type DemandPackage struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	RawDemand          string   `json:"raw_demand"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Assumptions        []string `json:"assumptions,omitempty"`
	NonGoals           []string `json:"non_goals,omitempty"`
}

// Validate returns an error when the package is not actionable.
func (p DemandPackage) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if p.Title == "" {
		return fmt.Errorf("demand package title is required")
	}
	if p.RawDemand == "" {
		return fmt.Errorf("raw demand is required")
	}
	if len(p.AcceptanceCriteria) == 0 {
		return fmt.Errorf("at least one acceptance criterion is required")
	}
	return nil
}
