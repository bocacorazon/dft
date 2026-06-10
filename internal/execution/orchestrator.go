package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// Orchestrator executes a WBS DAG by dispatching each spec to the executor
// selected for its lane, asking each executor whether the spec is already
// complete before running it so interrupted runs resume cleanly.
type Orchestrator struct {
	Registry     *ExecutorRegistry
	Git          ports.GitPort
	ArtifactRoot string
	HTTPClient   *http.Client
}

// Execute runs the WBS in dependency order and records the orchestration result.
func (o Orchestrator) Execute(ctx context.Context, runID string, request domain.OrchestrationRequest) (domain.ExecutionOrchestrationResult, error) {
	result := domain.ExecutionOrchestrationResult{
		RunID:   runID,
		WBSPath: request.WBSPath,
		Status:  "failed",
	}
	if err := request.Validate(); err != nil {
		return result, err
	}
	if o.Registry == nil {
		return result, fmt.Errorf("executor registry is required")
	}
	root := o.root()
	wbs, err := loadWBS(root, request.WBSPath)
	if err != nil {
		return result, err
	}
	if err := wbs.Validate(); err != nil {
		return result, err
	}
	baseBranch, incrementBranch, err := o.resolveBranches(ctx, request, wbs)
	if err != nil {
		return result, err
	}
	result.BaseBranch = baseBranch
	result.IncrementBranch = incrementBranch
	completed := map[string]bool{}
	done := map[string]bool{}
	for len(done) < len(wbs.Specs) {
		ready := readySpecs(wbs.Specs, done, completed)
		if len(ready) == 0 {
			return result, fmt.Errorf("wbs contains a dependency cycle or blocked specs")
		}
		for _, spec := range ready {
			executor, ok := o.Registry.For(spec.Lane)
			if !ok {
				return result, fmt.Errorf("no executor registered for lane %q (spec %q)", spec.Lane, spec.ID)
			}
			execRunID := runID + "--" + spec.ID
			execRequest := domain.ExecutionRequest{
				Prompt:          spec.Description,
				PromptPath:      spec.PromptPath,
				Workflow:        firstNonEmpty(spec.Workflow, request.Workflow),
				WorkflowPath:    firstNonEmpty(spec.WorkflowPath, request.WorkflowPath),
				FeatureSlug:     firstNonEmpty(spec.FeatureSlug, spec.ID),
				BaseBranch:      baseBranch,
				IncrementBranch: incrementBranch,
				ModelConfigPath: request.ModelConfigPath,
				ModelFamily:     firstNonEmpty(spec.ModelFamily, request.ModelFamily),
			}
			execResult, execErr := o.runSpec(ctx, executor, execRunID, spec.ID, execRequest)
			result.SpecResults = append(result.SpecResults, execResult)
			done[spec.ID] = true
			if execErr == nil {
				completed[spec.ID] = true
			}
			if err := o.notify(ctx, request.CompletionURL, domain.CallbackPayload{
				Event:           "spec.completed",
				RunID:           runID,
				ExecutionRunID:  execRunID,
				SpecID:          spec.ID,
				FeatureSlug:     execResult.FeatureSlug,
				BaseBranch:      execResult.BaseBranch,
				IncrementBranch: execResult.IncrementBranch,
				Status:          execResult.Status,
				Error:           execResult.Error,
			}); err != nil {
				return result, err
			}
			if execErr != nil {
				result.CompletedAt = time.Now().UTC()
				_ = writeJSON(filepath.Join(root, ".dft", "runs", runID, "execution-orchestration.json"), result)
				if callbackErr := o.notify(ctx, request.CompletionURL, domain.CallbackPayload{
					Event:           "run.completed",
					RunID:           runID,
					BaseBranch:      baseBranch,
					IncrementBranch: incrementBranch,
					Status:          "failed",
					Error:           execErr.Error(),
				}); callbackErr != nil {
					return result, callbackErr
				}
				return result, execErr
			}
		}
	}
	result.Status = "completed"
	result.CompletedAt = time.Now().UTC()
	if err := writeJSON(filepath.Join(root, ".dft", "runs", runID, "execution-orchestration.json"), result); err != nil {
		return result, err
	}
	if err := o.notify(ctx, request.CompletionURL, domain.CallbackPayload{
		Event:           "run.completed",
		RunID:           runID,
		BaseBranch:      baseBranch,
		IncrementBranch: incrementBranch,
		Status:          "completed",
	}); err != nil {
		return result, err
	}
	return result, nil
}

func (o Orchestrator) runSpec(ctx context.Context, executor Executor, execRunID string, specID string, request domain.ExecutionRequest) (domain.ExecutionResult, error) {
	if status, err := executor.Status(ctx, execRunID, specID, request); err == nil && status.Completed {
		return domain.ExecutionResult{
			RunID:           execRunID,
			SpecID:          specID,
			FeatureSlug:     request.FeatureSlug,
			BaseBranch:      request.BaseBranch,
			IncrementBranch: request.IncrementBranch,
			Status:          "completed",
			CompletedAt:     time.Now().UTC(),
		}, nil
	}
	return executor.Execute(ctx, execRunID, specID, request)
}

func (o Orchestrator) resolveBranches(ctx context.Context, request domain.OrchestrationRequest, wbs domain.WBS) (string, string, error) {
	baseBranch := firstNonEmpty(request.BaseBranch, wbs.BaseBranch)
	if strings.TrimSpace(baseBranch) == "" {
		defaultBranch, err := o.defaultBranch(ctx)
		if err != nil {
			return "", "", err
		}
		baseBranch = defaultBranch
	}
	incrementBranch := firstNonEmpty(request.IncrementBranch, wbs.IncrementBranch, baseBranch)
	if err := domain.ValidateBranchRef("base branch", baseBranch); err != nil {
		return "", "", err
	}
	if err := domain.ValidateBranchRef("increment branch", incrementBranch); err != nil {
		return "", "", err
	}
	return baseBranch, incrementBranch, nil
}

func (o Orchestrator) defaultBranch(ctx context.Context) (string, error) {
	git := o.Git
	if git == nil {
		return "", fmt.Errorf("base branch is required when git default branch cannot be resolved")
	}
	branch, err := git.DefaultBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve default branch: %w", err)
	}
	if err := domain.ValidateBranchRef("default branch", branch); err != nil {
		return "", err
	}
	return branch, nil
}

func (o Orchestrator) notify(ctx context.Context, callbackURL string, payload domain.CallbackPayload) error {
	if strings.TrimSpace(callbackURL) == "" {
		return nil
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode callback payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post callback: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callback returned status %d", resp.StatusCode)
	}
	return nil
}

func readySpecs(specs []domain.SpecRef, done map[string]bool, completed map[string]bool) []domain.SpecRef {
	var ready []domain.SpecRef
	for _, spec := range specs {
		if done[spec.ID] {
			continue
		}
		blocked := false
		for _, dep := range spec.DependsOn {
			if !completed[dep] {
				blocked = true
				break
			}
		}
		if !blocked {
			ready = append(ready, spec)
		}
	}
	return ready
}

func loadWBS(root string, path string) (domain.WBS, error) {
	content, err := os.ReadFile(resolvePath(root, path))
	if err != nil {
		return domain.WBS{}, fmt.Errorf("read wbs: %w", err)
	}
	var wbs domain.WBS
	if err := json.Unmarshal(content, &wbs); err != nil {
		return domain.WBS{}, fmt.Errorf("parse wbs: %w", err)
	}
	return wbs, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (o Orchestrator) root() string {
	if strings.TrimSpace(o.ArtifactRoot) == "" {
		return "."
	}
	return o.ArtifactRoot
}
