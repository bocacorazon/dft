package v2

import (
	"time"

	"github.com/bocacorazon/dft/internal/domain"
)

// SpecResult records the outcome of executing one spec via a lane.
type SpecResult struct {
	SpecID      string              `json:"spec_id"`
	Lane        string              `json:"lane"`       // which lane (executor) ran
	Status      string              `json:"status"`     // "completed" | "failed" | "skipped"
	Branch      string              `json:"branch"`
	Artifacts   []domain.ArtifactRef `json:"artifacts"`
	Transcripts []string            `json:"transcripts"` // relative paths
	Findings    []domain.Finding    `json:"findings,omitempty"`
}

// OrchestrationResult is the contract output of the Build phase.
type OrchestrationResult struct {
	DemandPackageID  string                 `json:"demand_package_id"`
	IncrementBranch  string                 `json:"increment_branch"`
	SpecResults      []SpecResult           `json:"spec_results"`
	ArtifactManifest domain.ArtifactManifest `json:"artifact_manifest"`
	CompletedAt      time.Time              `json:"completed_at"`
}