package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	taskLinePattern       = regexp.MustCompile(`(?m)^\s*[-*]\s+\[(?: |X|x)\]\s+`)
	completedTaskPattern  = regexp.MustCompile(`(?m)^\s*[-*]\s+\[[Xx]\]\s+`)
	incompleteTaskPattern = regexp.MustCompile(`(?m)^\s*[-*]\s+\[ \]\s+`)
)

// TaskChecklistStatus summarizes markdown checkbox completion in tasks.md.
type TaskChecklistStatus struct {
	Total      int
	Completed  int
	Incomplete int
}

// AllCompleted reports whether the checklist contains at least one task and every task is checked off.
func (s TaskChecklistStatus) AllCompleted() bool {
	return s.Total > 0 && s.Completed == s.Total && s.Incomplete == 0
}

// ReadTaskChecklistStatus parses a tasks.md file and reports checkbox completion counts.
func ReadTaskChecklistStatus(path string) (TaskChecklistStatus, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return TaskChecklistStatus{}, err
	}
	return summarizeTaskChecklist(content), nil
}

func verifySpeckitCommandArtifacts(root string, step Step) (map[string]any, error) {
	if !strings.HasPrefix(step.CommandName, "speckit.") {
		return nil, nil
	}
	projectRoot, err := resolveCommandProjectRoot(root, step)
	if err != nil {
		return nil, err
	}
	featureDir, err := resolveFeatureDirectory(projectRoot, step)
	if err != nil {
		return nil, err
	}
	switch step.CommandName {
	case "speckit.specify":
		specFile := filepath.Join(featureDir, "spec.md")
		checklistFile := filepath.Join(featureDir, "checklists", "requirements.md")
		if err := requirePaths(specFile, checklistFile); err != nil {
			return nil, fmt.Errorf("speckit.specify missing expected artifacts under %s: %w", filepath.ToSlash(featureDir), err)
		}
		return map[string]any{
			"feature_directory": filepath.ToSlash(featureDir),
			"spec_file":         filepath.ToSlash(specFile),
			"requirements_file": filepath.ToSlash(checklistFile),
		}, nil
	case "speckit.plan":
		planFile := filepath.Join(featureDir, "plan.md")
		researchFile := filepath.Join(featureDir, "research.md")
		contractsDir := filepath.Join(featureDir, "contracts")
		if err := requirePaths(planFile, researchFile); err != nil {
			return nil, fmt.Errorf("speckit.plan missing expected artifacts under %s: %w", filepath.ToSlash(featureDir), err)
		}
		return map[string]any{
			"feature_directory": filepath.ToSlash(featureDir),
			"plan_file":         filepath.ToSlash(planFile),
			"research_file":     filepath.ToSlash(researchFile),
			"contracts_dir":     filepath.ToSlash(contractsDir),
		}, nil
	case "speckit.tasks":
		tasksFile := filepath.Join(featureDir, "tasks.md")
		if err := requirePaths(tasksFile); err != nil {
			return nil, fmt.Errorf("speckit.tasks missing expected artifacts under %s: %w", filepath.ToSlash(featureDir), err)
		}
		return map[string]any{
			"feature_directory": filepath.ToSlash(featureDir),
			"tasks_file":        filepath.ToSlash(tasksFile),
		}, nil
	case "speckit.implement":
		tasksFile := filepath.Join(featureDir, "tasks.md")
		if err := requirePaths(tasksFile); err != nil {
			return nil, fmt.Errorf("speckit.implement missing expected tasks.md under %s: %w", filepath.ToSlash(featureDir), err)
		}
		status, err := ReadTaskChecklistStatus(tasksFile)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(tasksFile), err)
		}
		if status.Completed == 0 {
			return nil, fmt.Errorf("speckit.implement did not mark any tasks completed in %s", filepath.ToSlash(tasksFile))
		}
		return map[string]any{
			"feature_directory": filepath.ToSlash(featureDir),
			"tasks_file":        filepath.ToSlash(tasksFile),
			"completed_tasks":   status.AllCompleted(),
			"task_progress":     true,
		}, nil
	default:
		return nil, nil
	}
}

func resolveCommandProjectRoot(root string, step Step) (string, error) {
	base := root
	if base == "" {
		base = "."
	}
	base, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	if step.Cwd == "" {
		return filepath.Clean(base), nil
	}
	if filepath.IsAbs(step.Cwd) {
		return filepath.Clean(step.Cwd), nil
	}
	return filepath.Clean(filepath.Join(base, step.Cwd)), nil
}

func resolveFeatureDirectory(projectRoot string, step Step) (string, error) {
	featureJSON := filepath.Join(projectRoot, ".specify", "feature.json")
	content, err := os.ReadFile(featureJSON)
	if err == nil {
		var payload struct {
			FeatureDirectory string `json:"feature_directory"`
		}
		if err := json.Unmarshal(content, &payload); err != nil {
			return "", fmt.Errorf("parse %s: %w", filepath.ToSlash(featureJSON), err)
		}
		if strings.TrimSpace(payload.FeatureDirectory) != "" {
			if filepath.IsAbs(payload.FeatureDirectory) {
				return filepath.Clean(payload.FeatureDirectory), nil
			}
			return filepath.Clean(filepath.Join(projectRoot, payload.FeatureDirectory)), nil
		}
	}
	if value := strings.TrimSpace(step.Env["SPECIFY_FEATURE_DIRECTORY"]); value != "" {
		if filepath.IsAbs(value) {
			return filepath.Clean(value), nil
		}
		return filepath.Clean(filepath.Join(projectRoot, value)), nil
	}
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("missing SPECIFY_FEATURE_DIRECTORY and .specify/feature.json")
		}
		return "", fmt.Errorf("read %s: %w", filepath.ToSlash(featureJSON), err)
	}
	var payload struct {
		FeatureDirectory string `json:"feature_directory"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return "", fmt.Errorf("parse %s: %w", filepath.ToSlash(featureJSON), err)
	}
	if strings.TrimSpace(payload.FeatureDirectory) == "" {
		return "", fmt.Errorf("%s does not contain feature_directory", filepath.ToSlash(featureJSON))
	}
	if filepath.IsAbs(payload.FeatureDirectory) {
		return filepath.Clean(payload.FeatureDirectory), nil
	}
	return filepath.Clean(filepath.Join(projectRoot, payload.FeatureDirectory)), nil
}

func requirePaths(paths ...string) error {
	var missing []string
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, filepath.ToSlash(path))
				continue
			}
			return err
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func summarizeTaskChecklist(content []byte) TaskChecklistStatus {
	return TaskChecklistStatus{
		Total:      len(taskLinePattern.FindAll(content, -1)),
		Completed:  len(completedTaskPattern.FindAll(content, -1)),
		Incomplete: len(incompleteTaskPattern.FindAll(content, -1)),
	}
}
