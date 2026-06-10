package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
	"github.com/bocacorazon/dft/internal/ports"
	"gopkg.in/yaml.v3"
)

// Service executes one frozen spec flow from explicit runtime inputs.
type Service struct {
	Agent        ports.AgentAdapter
	Dispatcher   ports.CommandDispatcher
	Verifier     ports.Verifier
	Git          ports.GitPort
	ArtifactRoot string
}

var _ Executor = Service{}

// Execute runs one workflow and records the execution result.
func (s Service) Execute(ctx context.Context, runID string, specID string, request domain.ExecutionRequest) (domain.ExecutionResult, error) {
	result := domain.ExecutionResult{
		RunID:       runID,
		SpecID:      specID,
		FeatureSlug: request.FeatureSlug,
		Status:      "failed",
	}
	if err := request.Validate(); err != nil {
		result.Error = err.Error()
		return result, err
	}
	baseBranch, incrementBranch, err := s.resolveBranches(ctx, request)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.BaseBranch = baseBranch
	result.IncrementBranch = incrementBranch
	root := s.root()
	prompt, err := loadPrompt(root, request)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	definition, workflowPath, family, err := s.loadExecutionDefinition(root, request)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	inputs := map[string]any{
		"prompt":           prompt,
		"feature_slug":     request.FeatureSlug,
		"base_branch":      baseBranch,
		"increment_branch": incrementBranch,
		"branch_name":      incrementBranch,
		"spec_id":          specID,
		"run_id":           runID,
		"artifact_root":    root,
	}
	definition = flow.BindInputs(definition, inputs)
	definition = flow.BindDefinition(definition, flow.ExecutionContext{
		Cwd: root,
		Env: map[string]string{
			"GIT_BRANCH_NAME":      incrementBranch,
			"DFT_FEATURE_SLUG":     request.FeatureSlug,
			"DFT_BASE_BRANCH":      baseBranch,
			"DFT_INCREMENT_BRANCH": incrementBranch,
		},
	})
	runner := flow.Runner{
		Agent:            s.Agent,
		Dispatcher:       s.dispatcher(),
		ArtifactRoot:     root,
		RunID:            runID,
		Verifier:         s.Verifier,
		AutoApproveGates: true,
		Inputs:           inputs,
	}
	_, err = runner.Execute(ctx, definition)
	result.WorkflowPath = workflowPath
	result.ModelFamily = family
	result.CompletedAt = time.Now().UTC()
	if err != nil {
		result.Error = err.Error()
		_ = writeJSON(filepath.Join(root, ".dft", "runs", runID, "execution-result.json"), result)
		return result, err
	}
	result.Status = "completed"
	if err := writeJSON(filepath.Join(root, ".dft", "runs", runID, "execution-result.json"), result); err != nil {
		return result, err
	}
	return result, nil
}

// Status reports completeness for a generic flow execution from artifact truth:
// the spec is complete when its execution-result.json records a completed run.
func (s Service) Status(_ context.Context, runID string, specID string, _ domain.ExecutionRequest) (domain.ExecutionStatus, error) {
	status := domain.ExecutionStatus{RunID: runID, SpecID: specID}
	result, err := loadExecutionResult(s.root(), runID)
	if err != nil {
		status.Detail = "no execution result recorded"
		return status, nil
	}
	status.Completed = result.Status == "completed"
	if status.Completed {
		status.Detail = "execution-result.json reports completed"
	} else {
		status.Detail = "execution-result.json reports " + result.Status
	}
	return status, nil
}

func (s Service) resolveBranches(ctx context.Context, request domain.ExecutionRequest) (string, string, error) {
	baseBranch := strings.TrimSpace(request.BaseBranch)
	incrementBranch := strings.TrimSpace(request.IncrementBranch)
	if incrementBranch == "" {
		if baseBranch != "" {
			incrementBranch = baseBranch
		} else {
			defaultBranch, err := s.defaultBranch(ctx)
			if err != nil {
				return "", "", err
			}
			incrementBranch = defaultBranch
		}
	}
	if baseBranch == "" {
		baseBranch = incrementBranch
	}
	if err := domain.ValidateBranchRef("base branch", baseBranch); err != nil {
		return "", "", err
	}
	if err := domain.ValidateBranchRef("increment branch", incrementBranch); err != nil {
		return "", "", err
	}
	return baseBranch, incrementBranch, nil
}

func (s Service) defaultBranch(ctx context.Context) (string, error) {
	if s.Git == nil {
		return "", fmt.Errorf("increment branch is required when git default branch cannot be resolved")
	}
	branch, err := s.Git.DefaultBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve default branch: %w", err)
	}
	if err := domain.ValidateBranchRef("default branch", branch); err != nil {
		return "", err
	}
	return branch, nil
}

func (s Service) loadExecutionDefinition(root string, request domain.ExecutionRequest) (flow.Definition, string, string, error) {
	workflowPath, err := resolveWorkflowPath(root, request.Workflow, request.WorkflowPath)
	if err != nil {
		return flow.Definition{}, "", "", err
	}
	definition, err := flow.LoadDefinition(workflowPath)
	if err != nil {
		return flow.Definition{}, "", "", err
	}
	config, err := loadModelConfig(root, request.ModelConfigPath)
	if err != nil {
		return flow.Definition{}, "", "", err
	}
	family := strings.TrimSpace(request.ModelFamily)
	if family == "" {
		family = strings.TrimSpace(config.DefaultFamily)
	}
	resolved, err := flow.ResolveModels(definition, config, family)
	if err != nil {
		return flow.Definition{}, "", "", err
	}
	return resolved, workflowPath, family, nil
}

func (s Service) root() string {
	if strings.TrimSpace(s.ArtifactRoot) == "" {
		return "."
	}
	return s.ArtifactRoot
}

func (s Service) dispatcher() ports.CommandDispatcher {
	if s.Dispatcher != nil {
		return s.Dispatcher
	}
	if dispatcher, ok := s.Agent.(ports.CommandDispatcher); ok {
		return dispatcher
	}
	if s.Agent == nil {
		return nil
	}
	return agentCommandDispatcher{agent: s.Agent}
}

type agentCommandDispatcher struct {
	agent ports.AgentAdapter
}

func (d agentCommandDispatcher) DispatchCommand(ctx context.Context, request ports.CommandRequest) (ports.CommandResponse, error) {
	response, err := d.agent.Invoke(ctx, ports.AgentRequest{
		AgentName:  request.Command + ".agent.md",
		Prompt:     request.Input,
		Increment:     request.Input,
		RunID:      request.RunID,
		Cwd:        request.Cwd,
		Env:        request.Env,
		Model:      request.Model,
		AllowTools: request.AllowTools,
	})
	if err != nil {
		return ports.CommandResponse{}, err
	}
	return ports.CommandResponse{Stdout: response.Raw, ExitCode: 0}, nil
}

func resolveWorkflowPath(root string, workflow string, workflowPath string) (string, error) {
	if strings.TrimSpace(workflow) != "" && strings.TrimSpace(workflowPath) != "" {
		return "", fmt.Errorf("provide only one of workflow or workflow_path")
	}
	if strings.TrimSpace(workflowPath) != "" {
		return resolvePath(root, workflowPath), nil
	}
	name := strings.TrimSpace(workflow)
	if name == "" {
		return "", fmt.Errorf("workflow or workflow_path is required")
	}
	candidates := []string{
		filepath.Join(root, ".dft", "flows", name),
		filepath.Join(root, ".dft", "flows", name+".yaml"),
		filepath.Join(root, ".dft", "flows", name+".yml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("workflow %q not found under .dft/flows", name)
}

func loadPrompt(root string, request domain.ExecutionRequest) (string, error) {
	if text := strings.TrimSpace(request.Prompt); text != "" {
		return text, nil
	}
	path := resolvePath(root, request.PromptPath)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read prompt file: %w", err)
	}
	return string(content), nil
}

func loadModelConfig(root string, configPath string) (domain.ModelConfig, error) {
	path := resolvePath(root, configPath)
	content, err := os.ReadFile(path)
	if err != nil {
		return domain.ModelConfig{}, fmt.Errorf("read model config: %w", err)
	}
	var config domain.ModelConfig
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &config); err != nil {
			return domain.ModelConfig{}, fmt.Errorf("parse model config: %w", err)
		}
	default:
		if err := json.Unmarshal(content, &config); err != nil {
			return domain.ModelConfig{}, fmt.Errorf("parse model config: %w", err)
		}
	}
	return config, nil
}

func resolvePath(root string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func loadExecutionResult(root string, runID string) (domain.ExecutionResult, error) {
	path := filepath.Join(root, ".dft", "runs", runID, "execution-result.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	var result domain.ExecutionResult
	if err := json.Unmarshal(content, &result); err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("parse execution result: %w", err)
	}
	return result, nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s parent: %w", filepath.Base(path), err)
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
