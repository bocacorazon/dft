package orchestration

import "github.com/bocacorazon/dft/internal/domain"

// SpecKitLaneSummary exposes the operator-facing state of one spec lane.
type SpecKitLaneSummary struct {
	SpecID                string         `json:"spec_id"`
	LatestSuccessfulStage SpecKitStageID `json:"latest_successful_stage,omitempty"`
	BlockingStage         SpecKitStageID `json:"blocking_stage,omitempty"`
	LatestFindingsSummary map[string]any `json:"latest_findings_summary,omitempty"`
	AutomaticResumeSafe   bool           `json:"automatic_resume_safe"`
	ResumeRecommendation  string         `json:"resume_recommendation,omitempty"`
}

// SummarizeSpecKitLane derives operator-facing lane status from artifact truth.
func SummarizeSpecKitLane(artifactRoot string, runID string, spec domain.SpecRef, worktree SpecWorktree) (SpecKitLaneSummary, error) {
	state, err := assessSpecKitLaneState(artifactRoot, runID, spec, worktree)
	if err != nil {
		return SpecKitLaneSummary{}, err
	}
	if !state.Observed {
		return SpecKitLaneSummary{}, nil
	}
	summary := SpecKitLaneSummary{
		SpecID:                spec.ID,
		LatestSuccessfulStage: state.LatestSuccessfulStage,
		BlockingStage:         state.BlockingStage,
		LatestFindingsSummary: state.LatestFindingsSummary,
		ResumeRecommendation:  state.ResumeRecommendation,
	}
	if !state.Completed {
		switch state.ResumeRecommendation {
		case "retry_stage", "rerun_stage", "start_stage":
			summary.AutomaticResumeSafe = true
		}
	}
	return summary, nil
}
