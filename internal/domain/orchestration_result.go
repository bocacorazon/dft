package domain

import (
	"fmt"
	"time"
)

// SpecResult records the outcome of executing one spec via a lane.
type SpecResult struct {
	SpecID      string        `json:"spec_id"`
	Lane        string        `json:"lane"`   // which lane (executor) ran
	Status      string        `json:"status"` // "completed" | "failed" | "skipped"
	Branch      string        `json:"branch"`
	Artifacts   []ArtifactRef `json:"artifacts"`
	Transcripts []string      `json:"transcripts"` // relative paths
	Findings    []Finding     `json:"findings,omitempty"`
}

// OrchestrationResult is the contract output of the Build phase.
type OrchestrationResult struct {
	IncrementPackageID string           `json:"increment_package_id"`
	IncrementBranch    string           `json:"increment_branch"`
	SpecResults        []SpecResult     `json:"spec_results"`
	ArtifactManifest   ArtifactManifest `json:"artifact_manifest"`
	CompletedAt        time.Time        `json:"completed_at"`
}

// Validate returns an error when the orchestration result is incomplete.
func (or OrchestrationResult) Validate() error {
	if or.IncrementPackageID == "" {
		return fmt.Errorf("increment package id is required")
	}
	if or.IncrementBranch == "" {
		return fmt.Errorf("increment branch is required")
	}
	return nil
}
