// Package v2 defines the contract types for dft's four-phase pipeline.
//
// Phases 1-2 (Intent, Solution) produce these contracts via Hermes agents.
// Phases 3-4 (Build, Evaluate) consume them via the dft CLI engine.
//
// Reused from parent domain package (not redefined here):
//   - domain.WBS, domain.SpecRef, domain.LaneAssignment (wbs.go)
//   - domain.EvalSurfaceContract, domain.ArtifactManifest, domain.Finding (eval.go, verification.go)
package v2

import "time"

// AcceptanceCriterion is one atomic, testable requirement.
type AcceptanceCriterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Requirement string `json:"requirement,omitempty"` // links to REQ-xxx
}

// AmbiguityFinding records something unclear in the raw demand.
type AmbiguityFinding struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Resolution  string `json:"resolution,omitempty"` // filled after refinement
}

// DemandPackage is the contract output of the Intent phase.
// This is a v2 extension of the existing domain.DemandPackage,
// adding RefinedDemand, Ambiguities, and VerifiedComplete.
type DemandPackage struct {
	ID                 string              `json:"id"`
	Title              string              `json:"title"`
	RawDemand          string              `json:"raw_demand"`
	RefinedDemand      string              `json:"refined_demand"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
	Assumptions        []string            `json:"assumptions,omitempty"`
	NonGoals           []string            `json:"non_goals,omitempty"`
	Ambiguities        []AmbiguityFinding  `json:"ambiguities,omitempty"`
	VerifiedComplete   bool                `json:"verified_complete"`
	CreatedAt          time.Time           `json:"created_at"`
}