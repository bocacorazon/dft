package orchestration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
)

type specKitArtifactState struct {
	SpecID                string
	Observed              bool
	LatestSuccessfulStage SpecKitStageID
	BlockingStage         SpecKitStageID
	Status                SpecKitStageStatus
	ResumeStepID          string
	ResumeRecommendation  string
	LatestFindingsSummary map[string]any
	Completed             bool
}

type specKitWorkspace struct {
	FeatureDirectory string
	SpecFile         string
	PlanFile         string
	ResearchFile     string
	TasksFile        string
}

func assessSpecKitLaneState(artifactRoot string, runID string, spec domain.SpecRef, worktree SpecWorktree) (specKitArtifactState, error) {
	root := artifactRoot
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return specKitArtifactState{}, fmt.Errorf("resolve artifact root: %w", err)
	}
	workspace, err := resolveSpecKitWorkspace(root, spec.ID, worktree)
	if err != nil {
		return specKitArtifactState{}, err
	}
	state := specKitArtifactState{
		SpecID:               spec.ID,
		ResumeRecommendation: "start_stage",
		Status:               SpecKitStagePending,
	}
	state.Observed = hasAnySpecKitArtifacts(workspace, worktree)

	specTemplate := filepath.Join(root, ".specify", "templates", "spec-template.md")
	planTemplate := filepath.Join(root, ".specify", "templates", "plan-template.md")
	tasksTemplate := filepath.Join(root, ".specify", "templates", "tasks-template.md")

	specReady, err := concreteMarkdownFile(workspace.SpecFile, specTemplate)
	if err != nil {
		return specKitArtifactState{}, err
	}
	if !specReady {
		state.BlockingStage = SpecKitStageSpecify
		state.ResumeStepID = "specify"
		return state, nil
	}
	state.LatestSuccessfulStage = SpecKitStageSpecify

	planReady, err := concreteMarkdownFile(workspace.PlanFile, planTemplate)
	if err != nil {
		return specKitArtifactState{}, err
	}
	researchReady, err := concreteMarkdownFile(workspace.ResearchFile, "")
	if err != nil {
		return specKitArtifactState{}, err
	}
	if !planReady || !researchReady {
		state.BlockingStage = SpecKitStagePlan
		state.ResumeStepID = "plan"
		return state, nil
	}
	state.LatestSuccessfulStage = SpecKitStagePlan

	tasksReady, err := concreteMarkdownFile(workspace.TasksFile, tasksTemplate)
	if err != nil {
		return specKitArtifactState{}, err
	}
	if !tasksReady {
		state.BlockingStage = SpecKitStageTasks
		state.ResumeStepID = "tasks"
		return state, nil
	}
	state.LatestSuccessfulStage = SpecKitStageTasks

	analyzePath := stepParsedPath(root, runID, "analyze")
	analyzeSummary, analyzeExists, err := loadStepSummary(analyzePath)
	if err != nil {
		return specKitArtifactState{}, err
	}
	if !analyzeExists {
		state.BlockingStage = SpecKitStageAnalyze
		state.ResumeStepID = "analyze"
		return state, nil
	}
	if blockingFindings(analyzeSummary) > 0 {
		state.LatestFindingsSummary = analyzeSummary
		remediated, err := fileUpdatedAfter(workspace.TasksFile, analyzePath)
		if err != nil {
			return specKitArtifactState{}, err
		}
		if !remediated {
			state.BlockingStage = SpecKitStageAnalyze
			state.Status = SpecKitStageBlocked
			state.ResumeStepID = "tasks-remediation"
			state.ResumeRecommendation = "remediate_and_resume"
			return state, nil
		}
	} else {
		state.LatestSuccessfulStage = SpecKitStageAnalyze
	}

	checklist, err := flow.ReadTaskChecklistStatus(workspace.TasksFile)
	if err != nil {
		return specKitArtifactState{}, fmt.Errorf("read task checklist %s: %w", filepath.ToSlash(workspace.TasksFile), err)
	}
	if !checklist.AllCompleted() {
		state.BlockingStage = SpecKitStageImplement
		state.ResumeStepID = "implement-review-loop"
		return state, nil
	}
	state.LatestSuccessfulStage = SpecKitStageImplement

	reviewSummary, reviewExists, err := loadStepSummary(stepParsedPath(root, runID, "code-review"))
	if err != nil {
		return specKitArtifactState{}, err
	}
	if !reviewExists {
		state.BlockingStage = SpecKitStageCodeReview
		state.ResumeStepID = "implement-review-loop"
		return state, nil
	}
	if summaryCount(reviewSummary, "critical_findings") > 0 {
		state.BlockingStage = SpecKitStageCodeReview
		state.Status = SpecKitStageBlocked
		state.ResumeStepID = "implement-review-loop"
		state.ResumeRecommendation = "remediate_and_resume"
		state.LatestFindingsSummary = reviewSummary
		return state, nil
	}
	state.LatestSuccessfulStage = SpecKitStageCodeReview

	issuePath := stepParsedPath(root, runID, "issues-from-review")
	issueExists, err := pathExists(issuePath)
	if err != nil {
		return specKitArtifactState{}, err
	}
	if !issueExists {
		state.BlockingStage = SpecKitStageIssueHandoff
		state.ResumeStepID = "issues-from-review"
		return state, nil
	}
	state.LatestSuccessfulStage = SpecKitStageIssueHandoff

	if !worktreeHasGit(worktree.WorktreePath) || strings.TrimSpace(worktree.IncrementBranch) == "" {
		state.ResumeRecommendation = ""
		state.Status = SpecKitStageSucceeded
		state.Completed = true
		return state, nil
	}

	mergebackPath := stepParsedPath(root, runID, "mergeback-finalize")
	mergebackDocument, mergebackExists, err := loadStepDocument(mergebackPath)
	if err != nil {
		return specKitArtifactState{}, err
	}
	if !mergebackExists {
		mergebackAttempt, attemptExists, err := loadStepDocument(stepParsedPath(root, runID, "mergeback-attempt"))
		if err != nil {
			return specKitArtifactState{}, err
		}
		if attemptExists {
			switch strings.TrimSpace(fmt.Sprint(mergebackAttempt["status"])) {
			case "conflict":
				state.BlockingStage = SpecKitStageMergeback
				state.Status = SpecKitStageBlocked
				state.ResumeStepID = "resolve-mergeback"
				state.ResumeRecommendation = "resolve_conflicts"
				return state, nil
			case "rebased":
				state.BlockingStage = SpecKitStageMergeback
				state.ResumeStepID = "mergeback-finalize"
				return state, nil
			}
		}
		state.BlockingStage = SpecKitStageMergeback
		state.ResumeStepID = "commit-before-mergeback"
		return state, nil
	}
	if !boolPath(mergebackDocument, "trees_equal") ||
		!boolPath(mergebackDocument, "local_branch_deleted") ||
		!boolPath(mergebackDocument, "remote_branch_deleted_or_missing") {
		state.BlockingStage = SpecKitStageMergeback
		state.Status = SpecKitStageBlocked
		state.ResumeStepID = "commit-before-mergeback"
		state.ResumeRecommendation = "resolve_conflicts"
		return state, nil
	}
	state.LatestSuccessfulStage = SpecKitStageMergeback
	state.ResumeRecommendation = ""
	state.Status = SpecKitStageSucceeded
	state.Completed = true
	return state, nil
}

func resolveSpecKitWorkspace(root string, specID string, worktree SpecWorktree) (specKitWorkspace, error) {
	base := worktree.WorktreePath
	if strings.TrimSpace(base) == "" {
		base = filepath.Join(root, ".dft", "worktrees", worktree.RunID, specID)
	}
	base = filepath.Clean(base)
	featureDir := filepath.Join(base, "specs", specID)
	featureJSON := filepath.Join(base, ".specify", "feature.json")
	content, err := os.ReadFile(featureJSON)
	if err == nil {
		var payload struct {
			FeatureDirectory string `json:"feature_directory"`
		}
		if err := json.Unmarshal(content, &payload); err != nil {
			return specKitWorkspace{}, fmt.Errorf("parse %s: %w", filepath.ToSlash(featureJSON), err)
		}
		if strings.TrimSpace(payload.FeatureDirectory) != "" {
			if filepath.IsAbs(payload.FeatureDirectory) {
				featureDir = filepath.Clean(payload.FeatureDirectory)
			} else {
				featureDir = filepath.Clean(filepath.Join(base, payload.FeatureDirectory))
			}
		}
	} else if !os.IsNotExist(err) {
		return specKitWorkspace{}, fmt.Errorf("read %s: %w", filepath.ToSlash(featureJSON), err)
	}
	return specKitWorkspace{
		FeatureDirectory: featureDir,
		SpecFile:         filepath.Join(featureDir, "spec.md"),
		PlanFile:         filepath.Join(featureDir, "plan.md"),
		ResearchFile:     filepath.Join(featureDir, "research.md"),
		TasksFile:        filepath.Join(featureDir, "tasks.md"),
	}, nil
}

func concreteMarkdownFile(path string, templatePath string) (bool, error) {
	exists, err := pathExists(path)
	if err != nil || !exists {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", filepath.ToSlash(path), err)
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return false, nil
	}
	if templatePath != "" {
		templateExists, err := pathExists(templatePath)
		if err != nil {
			return false, err
		}
		if templateExists {
			same, err := sameChecksum(path, templatePath)
			if err != nil {
				return false, err
			}
			if same {
				return false, nil
			}
		}
	}
	if looksTemplated(trimmed) {
		return false, nil
	}
	return true, nil
}

func looksTemplated(content string) bool {
	upper := strings.ToUpper(content)
	for _, marker := range []string{
		"{{",
		"}}",
		"[NEEDS CLARIFICATION]",
		"ACTION REQUIRED",
		"TODO",
		"TBD",
		"[PLACEHOLDER",
		"<PLACEHOLDER",
	} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func hasAnySpecKitArtifacts(workspace specKitWorkspace, worktree SpecWorktree) bool {
	paths := []string{
		workspace.SpecFile,
		workspace.PlanFile,
		workspace.ResearchFile,
		workspace.TasksFile,
		filepath.Join(worktree.WorktreePath, ".specify", "feature.json"),
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if ok, _ := pathExists(path); ok {
			return true
		}
	}
	return false
}

func stepParsedPath(root string, runID string, stepID string) string {
	return filepath.Join(root, ".dft", "runs", runID, "steps", stepID, "parsed.json")
}

func loadStepSummary(path string) (map[string]any, bool, error) {
	document, exists, err := loadStepDocument(path)
	if err != nil || !exists {
		return nil, exists, err
	}
	summary, ok := document["summary"].(map[string]any)
	if !ok {
		return map[string]any{}, true, nil
	}
	return summary, true, nil
}

func loadStepDocument(path string) (map[string]any, bool, error) {
	exists, err := pathExists(path)
	if err != nil || !exists {
		return nil, exists, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, true, fmt.Errorf("read %s: %w", filepath.ToSlash(path), err)
	}
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", filepath.ToSlash(path), err)
	}
	return document, true, nil
}

func blockingFindings(summary map[string]any) int {
	return summaryCount(summary, "blocking_findings")
}

func summaryCount(summary map[string]any, key string) int {
	if len(summary) == 0 {
		return 0
	}
	value, ok := summary[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		count, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return count
		}
	}
	return 0
}

func boolPath(document map[string]any, key string) bool {
	value, ok := document[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	}
	return false
}

func fileUpdatedAfter(path string, reference string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	referenceInfo, err := os.Stat(reference)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.ModTime().After(referenceInfo.ModTime()), nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func sameChecksum(left string, right string) (bool, error) {
	leftHash, err := fileChecksum(left)
	if err != nil {
		return false, err
	}
	rightHash, err := fileChecksum(right)
	if err != nil {
		return false, err
	}
	return leftHash == rightHash, nil
}

func fileChecksum(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filepath.ToSlash(path), err)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}
