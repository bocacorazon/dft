package orchestration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
)

// SpecKitStageAttempt captures one durable stage attempt for a spec lane.
type SpecKitStageAttempt struct {
	Stage                SpecKitStageID             `json:"stage"`
	ExecutionID          string                     `json:"execution_id"`
	Attempt              int                        `json:"attempt"`
	Status               SpecKitStageStatus         `json:"status"`
	BlockingPolicy       SpecKitBlockingPolicy      `json:"blocking_policy"`
	PrimaryStepID        string                     `json:"primary_step_id"`
	FindingsStepID       string                     `json:"findings_step_id,omitempty"`
	RenderedChecks       []domain.Check             `json:"rendered_checks,omitempty"`
	Verification         *domain.VerificationResult `json:"verification,omitempty"`
	ArtifactPaths        map[string]string          `json:"artifact_paths,omitempty"`
	FindingsSummary      map[string]any             `json:"findings_summary,omitempty"`
	ResumeRecommendation string                     `json:"resume_recommendation,omitempty"`
}

// SpecKitLaneJournal is the durable spec-scoped journal for one lane run.
type SpecKitLaneJournal struct {
	SpecID   string                `json:"spec_id"`
	Attempts []SpecKitStageAttempt `json:"attempts"`
}

// AttemptForStage returns the latest attempt for a stage.
func (j SpecKitLaneJournal) AttemptForStage(stageID SpecKitStageID) (SpecKitStageAttempt, bool) {
	for i := len(j.Attempts) - 1; i >= 0; i-- {
		if j.Attempts[i].Stage == stageID {
			return j.Attempts[i], true
		}
	}
	return SpecKitStageAttempt{}, false
}

type specKitLaneJournalObserver struct {
	artifactRoot string
	runID        string
	specID       string
	journal      SpecKitLaneJournal
}

func newSpecKitLaneJournalObserver(artifactRoot string, runID string, specID string) (*specKitLaneJournalObserver, error) {
	observer := &specKitLaneJournalObserver{
		artifactRoot: artifactRoot,
		runID:        runID,
		specID:       specID,
		journal: SpecKitLaneJournal{
			SpecID: specID,
		},
	}
	content, err := os.ReadFile(observer.path())
	if err == nil {
		if err := json.Unmarshal(content, &observer.journal); err != nil {
			return nil, fmt.Errorf("parse lane journal: %w", err)
		}
		if observer.journal.SpecID == "" {
			observer.journal.SpecID = specID
		}
		return observer, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read lane journal: %w", err)
	}
	return observer, nil
}

func (o *specKitLaneJournalObserver) StepStarted(_ string, step flow.Step, _ flow.Result) {
	contract, ok := specKitStageContractForPrimaryStep(step.ID)
	if !ok {
		return
	}
	attempt := SpecKitStageAttempt{
		Stage:                contract.ID,
		ExecutionID:          contract.ExecutionID(o.specID, o.nextAttempt(contract.ID)),
		Attempt:              o.nextAttempt(contract.ID),
		Status:               SpecKitStageRunning,
		BlockingPolicy:       contract.BlockingPolicy,
		PrimaryStepID:        contract.PrimaryStepID,
		FindingsStepID:       contract.FindingsStepID,
		ResumeRecommendation: "wait_for_stage_completion",
	}
	o.journal.Attempts = append(o.journal.Attempts, attempt)
	_ = o.save()
}

func (o *specKitLaneJournalObserver) StepCompleted(_ string, step flow.Step, status flow.StepStatus, result flow.Result) {
	if contract, ok := specKitStageContractForPrimaryStep(step.ID); ok {
		if attempt := o.latestAttemptIndex(contract.ID); attempt >= 0 {
			if stepWasSkipped(result, step.ID) {
				o.journal.Attempts = append(o.journal.Attempts[:attempt], o.journal.Attempts[attempt+1:]...)
				_ = o.save()
				return
			}
			record := o.journal.Attempts[attempt]
			o.populateAttempt(&record, contract, step, status, result)
			if stageUsesSeparateVerification(contract, step) && status == flow.StepSucceeded {
				record.Status = SpecKitStageRunning
				record.ResumeRecommendation = "wait_for_stage_completion"
			}
			o.journal.Attempts[attempt] = record
			_ = o.save()
		}
		return
	}
	if contract, ok := specKitStageContractForVerificationStep(step); ok {
		if attempt := o.latestAttemptIndex(contract.ID); attempt >= 0 {
			record := o.journal.Attempts[attempt]
			o.populateAttempt(&record, contract, step, status, result)
			o.journal.Attempts[attempt] = record
			_ = o.save()
		}
	}
}

func (o *specKitLaneJournalObserver) populateAttempt(record *SpecKitStageAttempt, contract SpecKitStageContract, step flow.Step, status flow.StepStatus, result flow.Result) {
	record.RenderedChecks = append([]domain.Check(nil), renderedStageChecks(step)...)
	if verification, ok := latestVerificationForStage(result, contract.VerificationCheckIDs); ok {
		copy := verification
		record.Verification = &copy
	}
	if artifacts := artifactPaths(result, contract.PrimaryStepID); len(artifacts) > 0 {
		record.ArtifactPaths = artifacts
	}
	if summary := findingsSummary(result, contract.FindingsStepID); len(summary) > 0 {
		record.FindingsSummary = summary
	}
	record.Status = stageStatus(contract, step, status, result)
	record.ResumeRecommendation = resumeRecommendation(contract, record.Status)
}

func (o *specKitLaneJournalObserver) nextAttempt(stageID SpecKitStageID) int {
	count := 0
	for _, attempt := range o.journal.Attempts {
		if attempt.Stage == stageID {
			count++
		}
	}
	return count + 1
}

func (o *specKitLaneJournalObserver) latestAttemptIndex(stageID SpecKitStageID) int {
	for i := len(o.journal.Attempts) - 1; i >= 0; i-- {
		if o.journal.Attempts[i].Stage == stageID {
			return i
		}
	}
	return -1
}

func (o *specKitLaneJournalObserver) path() string {
	return filepath.Join(o.artifactRoot, ".dft", "runs", o.runID, "specs", o.specID, "lane-journal.json")
}

func (o *specKitLaneJournalObserver) save() error {
	if err := os.MkdirAll(filepath.Dir(o.path()), 0o755); err != nil {
		return fmt.Errorf("create lane journal directory: %w", err)
	}
	content, err := json.MarshalIndent(o.journal, "", "  ")
	if err != nil {
		return fmt.Errorf("encode lane journal: %w", err)
	}
	if err := os.WriteFile(o.path(), append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write lane journal: %w", err)
	}
	return nil
}

func specKitStageContractForPrimaryStep(stepID string) (SpecKitStageContract, bool) {
	for _, contract := range SpecKitStageContracts() {
		if contract.PrimaryStepID == stepID {
			return contract, true
		}
	}
	return SpecKitStageContract{}, false
}

func specKitStageContractForVerificationStep(step flow.Step) (SpecKitStageContract, bool) {
	for _, contract := range SpecKitStageContracts() {
		if len(contract.VerificationCheckIDs) == 0 {
			continue
		}
		if !stepContainsContractChecks(step, contract.VerificationCheckIDs) {
			continue
		}
		for _, controlStepID := range contract.ControlStepIDs {
			if step.ID == controlStepID {
				return contract, true
			}
		}
	}
	return SpecKitStageContract{}, false
}

func stageUsesSeparateVerification(contract SpecKitStageContract, step flow.Step) bool {
	if len(contract.VerificationCheckIDs) == 0 {
		return false
	}
	return !stepContainsContractChecks(step, contract.VerificationCheckIDs)
}

func stepContainsContractChecks(step flow.Step, checkIDs []string) bool {
	if len(checkIDs) == 0 {
		return false
	}
	available := renderedStageChecks(step)
	for _, availableCheck := range available {
		for _, checkID := range checkIDs {
			if availableCheck.ID == checkID {
				return true
			}
		}
	}
	return false
}

func renderedStageChecks(step flow.Step) []domain.Check {
	if len(step.Checks) > 0 {
		return step.Checks
	}
	return step.Verify
}

func latestVerificationForStage(result flow.Result, checkIDs []string) (domain.VerificationResult, bool) {
	if len(checkIDs) == 0 {
		return domain.VerificationResult{}, false
	}
	for i := len(result.Verification) - 1; i >= 0; i-- {
		for _, check := range result.Verification[i].Results {
			for _, wanted := range checkIDs {
				if check.CheckID == wanted {
					return result.Verification[i], true
				}
			}
		}
	}
	return domain.VerificationResult{}, false
}

func artifactPaths(result flow.Result, stepID string) map[string]string {
	if stepID == "" {
		return nil
	}
	output, ok := result.StepOutputs[stepID]
	if !ok {
		return nil
	}
	artifacts, ok := output["artifacts"].(map[string]any)
	if !ok {
		return nil
	}
	paths := map[string]string{}
	for key, value := range artifacts {
		text, ok := value.(string)
		if !ok || text == "" {
			continue
		}
		paths[key] = text
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

func findingsSummary(result flow.Result, stepID string) map[string]any {
	if stepID == "" {
		return nil
	}
	output, ok := result.StepOutputs[stepID]
	if !ok {
		return nil
	}
	summary, ok := output["summary"].(map[string]any)
	if !ok || len(summary) == 0 {
		return nil
	}
	return summary
}

func stageStatus(contract SpecKitStageContract, step flow.Step, status flow.StepStatus, result flow.Result) SpecKitStageStatus {
	switch status {
	case flow.StepPaused:
		return SpecKitStageBlocked
	case flow.StepFailed:
		if stepContainsContractChecks(step, contract.VerificationCheckIDs) && (contract.BlockingPolicy == SpecKitBlockingRemediate || contract.BlockingPolicy == SpecKitBlockingResolveConflicts) {
			return SpecKitStageBlocked
		}
		return SpecKitStageFailed
	case flow.StepSucceeded:
		if hasBlockingFindings(result, contract.FindingsStepID) {
			return SpecKitStageBlocked
		}
		return SpecKitStageSucceeded
	default:
		return SpecKitStageFailed
	}
}

func hasBlockingFindings(result flow.Result, stepID string) bool {
	if stepID == "" {
		return false
	}
	summary := findingsSummary(result, stepID)
	if len(summary) == 0 {
		return false
	}
	value, ok := summary["blocking_findings"]
	if !ok {
		return false
	}
	return fmt.Sprint(value) != "0"
}

func resumeRecommendation(contract SpecKitStageContract, status SpecKitStageStatus) string {
	switch status {
	case SpecKitStageSucceeded:
		return "next_stage"
	case SpecKitStageRunning:
		return "wait_for_stage_completion"
	case SpecKitStageBlocked:
		switch contract.BlockingPolicy {
		case SpecKitBlockingRetry:
			return "retry_stage"
		case SpecKitBlockingRemediate:
			return "remediate_and_resume"
		case SpecKitBlockingResolveConflicts:
			return "resolve_conflicts_and_resume"
		default:
			return "review_and_resume"
		}
	default:
		return "rerun_stage"
	}
}

func stepWasSkipped(result flow.Result, stepID string) bool {
	output, ok := result.StepOutputs[stepID]
	if !ok {
		return false
	}
	return fmt.Sprint(output["status"]) == "skipped"
}
