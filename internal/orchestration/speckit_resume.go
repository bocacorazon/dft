package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
)

// SpecKitResumeDecision describes where an interrupted lane should restart.
type SpecKitResumeDecision struct {
	SpecID               string             `json:"spec_id"`
	Stage                SpecKitStageID     `json:"stage,omitempty"`
	Status               SpecKitStageStatus `json:"status,omitempty"`
	ResumeStepID         string             `json:"resume_step_id,omitempty"`
	StepIndex            int                `json:"step_index,omitempty"`
	ResumeRecommendation string             `json:"resume_recommendation,omitempty"`
	Completed            bool               `json:"completed,omitempty"`
}

// LoadSpecKitLaneJournal reads the durable journal for one spec lane run.
func LoadSpecKitLaneJournal(root string, runID string, specID string) (SpecKitLaneJournal, error) {
	path := filepath.Join(root, ".dft", "runs", runID, "specs", specID, "lane-journal.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return SpecKitLaneJournal{}, fmt.Errorf("read lane journal: %w", err)
	}
	var journal SpecKitLaneJournal
	if err := json.Unmarshal(content, &journal); err != nil {
		return SpecKitLaneJournal{}, fmt.Errorf("parse lane journal: %w", err)
	}
	if journal.SpecID == "" {
		journal.SpecID = specID
	}
	return journal, nil
}

// DecideSpecKitLaneResume chooses the next stage from artifact truth rather than the journal.
func DecideSpecKitLaneResume(definition flow.Definition, artifactRoot string, runID string, spec domain.SpecRef, worktree SpecWorktree) (SpecKitResumeDecision, error) {
	state, err := assessSpecKitLaneState(artifactRoot, runID, spec, worktree)
	if err != nil {
		return SpecKitResumeDecision{}, err
	}
	decision := SpecKitResumeDecision{
		SpecID:               spec.ID,
		Stage:                state.BlockingStage,
		Status:               state.Status,
		ResumeStepID:         state.ResumeStepID,
		ResumeRecommendation: state.ResumeRecommendation,
		Completed:            state.Completed,
	}
	if decision.Completed {
		return decision, nil
	}
	if decision.ResumeStepID == "" {
		decision.Stage = SpecKitStageSpecify
		decision.Status = SpecKitStagePending
		decision.ResumeStepID = "specify"
		decision.ResumeRecommendation = "start_stage"
	}
	stepIndex, err := topLevelStepIndex(definition, decision.ResumeStepID)
	if err != nil {
		return SpecKitResumeDecision{}, err
	}
	decision.StepIndex = stepIndex
	return decision, nil
}

// ResumeSpecKitLane executes the spec lane from the journal-selected stage.
func ResumeSpecKitLane(ctx context.Context, artifactRoot string, runID string, spec domain.SpecRef, worktree SpecWorktree, runner flow.Runner) (SpecKitResumeDecision, flow.Result, error) {
	definition, err := LoadSpecKitLane(artifactRoot, spec, worktree)
	if err != nil {
		return SpecKitResumeDecision{}, flow.Result{}, fmt.Errorf("load spec workflow %s: %w", spec.ID, err)
	}
	observer, err := newSpecKitLaneJournalObserver(artifactRoot, runID, spec.ID)
	if err != nil {
		return SpecKitResumeDecision{}, flow.Result{}, fmt.Errorf("create lane journal observer for spec %s: %w", spec.ID, err)
	}
	runner.Observer = observer
	decision, err := DecideSpecKitLaneResume(definition, artifactRoot, runID, spec, worktree)
	if err != nil {
		return SpecKitResumeDecision{}, flow.Result{}, err
	}
	if decision.Completed {
		return decision, flow.Result{}, nil
	}
	if decision.StepIndex == 0 {
		result, execErr := runner.Execute(ctx, definition)
		return decision, result, execErr
	}
	result, err := runner.ResumeFrom(ctx, definition, decision.StepIndex)
	return decision, result, err
}

func topLevelStepIndex(definition flow.Definition, stepID string) (int, error) {
	for i, step := range definition.Steps {
		if step.ID == stepID {
			return i, nil
		}
	}
	return 0, fmt.Errorf("resume step %q missing from top-level flow", stepID)
}
