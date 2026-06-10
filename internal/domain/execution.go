package domain

import (
	"fmt"
	"strings"
	"time"
)

// ModelTier names the abstract capacity tier used in workflow steps.
type ModelTier string

const (
	ModelTierXHigh  ModelTier = "xhigh"
	ModelTierHigh   ModelTier = "high"
	ModelTierMedium ModelTier = "medium"
	ModelTierLow    ModelTier = "low"
)

// ModelFamilyConfig maps abstract workflow tiers to concrete model IDs.
type ModelFamilyConfig struct {
	XHigh  string `json:"xhigh" yaml:"xhigh"`
	High   string `json:"high" yaml:"high"`
	Medium string `json:"medium" yaml:"medium"`
	Low    string `json:"low" yaml:"low"`
}

// ModelConfig is the runtime model catalog consumed by execution requests.
type ModelConfig struct {
	DefaultFamily string                       `json:"default_family,omitempty" yaml:"default_family"`
	Families      map[string]ModelFamilyConfig `json:"families" yaml:"families"`
}

// Resolve returns the concrete model ID for a family/tier pair.
func (c ModelConfig) Resolve(family string, tier ModelTier) (string, error) {
	if len(c.Families) == 0 {
		return "", fmt.Errorf("at least one model family is required")
	}
	resolvedFamily := strings.TrimSpace(family)
	if resolvedFamily == "" {
		resolvedFamily = strings.TrimSpace(c.DefaultFamily)
	}
	if resolvedFamily == "" {
		if len(c.Families) != 1 {
			return "", fmt.Errorf("model family is required when model config has multiple families")
		}
		for name := range c.Families {
			resolvedFamily = name
		}
	}
	familyConfig, ok := c.Families[resolvedFamily]
	if !ok {
		return "", fmt.Errorf("unknown model family %q", resolvedFamily)
	}
	switch tier {
	case ModelTierXHigh:
		if strings.TrimSpace(familyConfig.XHigh) == "" {
			return "", fmt.Errorf("model family %q is missing xhigh", resolvedFamily)
		}
		return familyConfig.XHigh, nil
	case ModelTierHigh:
		if strings.TrimSpace(familyConfig.High) == "" {
			return "", fmt.Errorf("model family %q is missing high", resolvedFamily)
		}
		return familyConfig.High, nil
	case ModelTierMedium:
		if strings.TrimSpace(familyConfig.Medium) == "" {
			return "", fmt.Errorf("model family %q is missing medium", resolvedFamily)
		}
		return familyConfig.Medium, nil
	case ModelTierLow:
		if strings.TrimSpace(familyConfig.Low) == "" {
			return "", fmt.Errorf("model family %q is missing low", resolvedFamily)
		}
		return familyConfig.Low, nil
	default:
		return "", fmt.Errorf("unsupported model tier %q", tier)
	}
}

// ValidateBranchRef validates a git branch-like reference used by execution contracts.
func ValidateBranchRef(name string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.Contains(trimmed, "..") || strings.ContainsAny(trimmed, " \t\n\\") {
		return fmt.Errorf("%s %q contains unsupported characters", name, value)
	}
	return nil
}

// ExecutionRequest is the execution-first contract for one frozen spec flow.
type ExecutionRequest struct {
	Prompt          string `json:"prompt,omitempty"`
	PromptPath      string `json:"prompt_path,omitempty"`
	Workflow        string `json:"workflow,omitempty"`
	WorkflowPath    string `json:"workflow_path,omitempty"`
	FeatureSlug     string `json:"feature_slug"`
	BaseBranch      string `json:"base_branch,omitempty"`
	IncrementBranch string `json:"increment_branch,omitempty"`
	ModelConfigPath string `json:"model_config_path"`
	ModelFamily     string `json:"model_family,omitempty"`
}

// Validate ensures the request fully specifies one executable spec.
func (r ExecutionRequest) Validate() error {
	if (strings.TrimSpace(r.Prompt) == "") == (strings.TrimSpace(r.PromptPath) == "") {
		return fmt.Errorf("exactly one of prompt or prompt_path is required")
	}
	if (strings.TrimSpace(r.Workflow) == "") == (strings.TrimSpace(r.WorkflowPath) == "") {
		return fmt.Errorf("exactly one of workflow or workflow_path is required")
	}
	if strings.TrimSpace(r.FeatureSlug) == "" {
		return fmt.Errorf("feature slug is required")
	}
	if strings.TrimSpace(r.BaseBranch) != "" {
		if err := ValidateBranchRef("base branch", r.BaseBranch); err != nil {
			return err
		}
	}
	if strings.TrimSpace(r.IncrementBranch) != "" {
		if err := ValidateBranchRef("increment branch", r.IncrementBranch); err != nil {
			return err
		}
	}
	if strings.TrimSpace(r.ModelConfigPath) == "" {
		return fmt.Errorf("model config path is required")
	}
	return nil
}

// OrchestrationRequest describes one WBS-driven execution run.
type OrchestrationRequest struct {
	WBSPath         string `json:"wbs_path"`
	Workflow        string `json:"workflow,omitempty"`
	WorkflowPath    string `json:"workflow_path,omitempty"`
	BaseBranch      string `json:"base_branch,omitempty"`
	IncrementBranch string `json:"increment_branch,omitempty"`
	ModelConfigPath string `json:"model_config_path"`
	ModelFamily     string `json:"model_family,omitempty"`
	CompletionURL   string `json:"completion_url,omitempty"`
}

// Validate ensures the orchestration request can dispatch specs.
func (r OrchestrationRequest) Validate() error {
	if strings.TrimSpace(r.WBSPath) == "" {
		return fmt.Errorf("wbs path is required")
	}
	if strings.TrimSpace(r.ModelConfigPath) == "" {
		return fmt.Errorf("model config path is required")
	}
	if strings.TrimSpace(r.BaseBranch) != "" {
		if err := ValidateBranchRef("base branch", r.BaseBranch); err != nil {
			return err
		}
	}
	if strings.TrimSpace(r.IncrementBranch) != "" {
		if err := ValidateBranchRef("increment branch", r.IncrementBranch); err != nil {
			return err
		}
	}
	if strings.TrimSpace(r.Workflow) != "" && strings.TrimSpace(r.WorkflowPath) != "" {
		return fmt.Errorf("provide only one of workflow or workflow_path")
	}
	return nil
}

// ExecutionResult captures the result of one single-spec execution.
type ExecutionResult struct {
	RunID           string    `json:"run_id"`
	SpecID          string    `json:"spec_id,omitempty"`
	FeatureSlug     string    `json:"feature_slug"`
	BaseBranch      string    `json:"base_branch,omitempty"`
	IncrementBranch string    `json:"increment_branch,omitempty"`
	WorkflowPath    string    `json:"workflow_path"`
	ModelFamily     string    `json:"model_family,omitempty"`
	Status          string    `json:"status"`
	Error           string    `json:"error,omitempty"`
	CompletedAt     time.Time `json:"completed_at"`
}

// ExecutionStatus reports a spec's completeness derived from artifact truth.
// The orchestrator queries it before dispatching so finished specs are skipped
// when an interrupted run resumes.
type ExecutionStatus struct {
	RunID       string `json:"run_id"`
	SpecID      string `json:"spec_id,omitempty"`
	Completed   bool   `json:"completed"`
	ResumePoint string `json:"resume_point,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

// ExecutionOrchestrationResult captures the full WBS orchestration result.
type ExecutionOrchestrationResult struct {
	RunID           string            `json:"run_id"`
	WBSPath         string            `json:"wbs_path"`
	BaseBranch      string            `json:"base_branch,omitempty"`
	IncrementBranch string            `json:"increment_branch,omitempty"`
	Status          string            `json:"status"`
	SpecResults     []ExecutionResult `json:"spec_results"`
	CompletedAt     time.Time         `json:"completed_at"`
}

// CallbackPayload is posted to the completion callback URL.
type CallbackPayload struct {
	Event           string `json:"event"`
	RunID           string `json:"run_id"`
	ExecutionRunID  string `json:"execution_run_id,omitempty"`
	SpecID          string `json:"spec_id,omitempty"`
	FeatureSlug     string `json:"feature_slug,omitempty"`
	BaseBranch      string `json:"base_branch,omitempty"`
	IncrementBranch string `json:"increment_branch,omitempty"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
}
