package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/ports"
)

// StepType names the kind of work a typed flow step performs.
type StepType string

const (
	StepAgent    StepType = "agent"
	StepFunction StepType = "function"
	StepVerify   StepType = "verify"
)

// StepStatus captures the terminal state of an executed step.
type StepStatus string

const (
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
)

// Definition is the minimal built-in flow shape used before external DSL support.
type Definition struct {
	MaxSpecParallelism int    `json:"max_spec_parallelism,omitempty"`
	Steps              []Step `json:"steps"`
}

// Step describes one typed flow step.
type Step struct {
	ID            string            `json:"id"`
	Type          StepType          `json:"type"`
	AgentName     string            `json:"agent_name,omitempty"`
	Prompt        string            `json:"prompt,omitempty"`
	Demand        string            `json:"demand,omitempty"`
	Cwd           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	MaxIterations int               `json:"max_iterations,omitempty"`
}

// Runner executes typed flow definitions and writes per-step audit artifacts.
type Runner struct {
	Agent        ports.AgentAdapter
	ArtifactRoot string
	RunID        string
}

// Result contains terminal status for every completed step.
type Result struct {
	Steps []StepResult
}

// StepResult contains terminal status for one step.
type StepResult struct {
	ID     string     `json:"id"`
	Type   StepType   `json:"type"`
	Status StepStatus `json:"status"`
}

// Execute runs each step sequentially and stops at the first failure.
func (r Runner) Execute(ctx context.Context, definition Definition) (Result, error) {
	if r.RunID == "" {
		return Result{}, fmt.Errorf("run id is required")
	}

	result := Result{Steps: make([]StepResult, 0, len(definition.Steps))}
	for _, step := range definition.Steps {
		stepResult, err := r.executeStep(ctx, step)
		result.Steps = append(result.Steps, stepResult)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r Runner) executeStep(ctx context.Context, step Step) (StepResult, error) {
	if step.ID == "" {
		return StepResult{Type: step.Type, Status: StepFailed}, fmt.Errorf("step id is required")
	}
	if step.Type == "" {
		return StepResult{ID: step.ID, Status: StepFailed}, fmt.Errorf("step %q type is required", step.ID)
	}

	stepDir := filepath.Join(r.ArtifactRoot, ".dft", "runs", r.RunID, "steps", step.ID)
	if err := os.MkdirAll(stepDir, 0o755); err != nil {
		return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("create step artifact directory: %w", err)
	}

	switch step.Type {
	case StepAgent:
		iterations := step.MaxIterations
		if iterations == 0 {
			iterations = 1
		}
		var lastErr error
		for i := 0; i < iterations; i++ {
			err := r.executeAgentStep(ctx, step, stepDir)
			if err == nil {
				return StepResult{ID: step.ID, Type: step.Type, Status: StepSucceeded}, nil
			}
			lastErr = err
		}
		return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, lastErr
	case StepFunction, StepVerify:
		if err := writeParsed(stepDir, map[string]string{"status": "not_implemented"}); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	default:
		return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("unsupported step type %q", step.Type)
	}

	return StepResult{ID: step.ID, Type: step.Type, Status: StepSucceeded}, nil
}

func (r Runner) executeAgentStep(ctx context.Context, step Step, stepDir string) error {
	if r.Agent == nil {
		return fmt.Errorf("agent adapter is required")
	}
	if step.AgentName == "" {
		return fmt.Errorf("step %q agent name is required", step.ID)
	}
	if err := os.WriteFile(filepath.Join(stepDir, "prompt.md"), []byte(step.Prompt), 0o644); err != nil {
		return fmt.Errorf("write prompt artifact: %w", err)
	}
	response, err := r.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: step.AgentName,
		Prompt:    step.Prompt,
		Demand:    step.Demand,
		RunID:     r.RunID,
		Cwd:       step.Cwd,
		Env:       step.Env,
	})
	if err != nil {
		return fmt.Errorf("invoke agent step %q: %w", step.ID, err)
	}
	if err := os.WriteFile(filepath.Join(stepDir, "stdout.txt"), []byte(response.Raw), 0o644); err != nil {
		return fmt.Errorf("write stdout artifact: %w", err)
	}
	var parsed any
	if err := json.Unmarshal([]byte(response.Raw), &parsed); err != nil {
		return fmt.Errorf("parse agent step %q output: %w", step.ID, err)
	}
	if err := writeParsed(stepDir, parsed); err != nil {
		return err
	}
	return nil
}

func writeParsed(stepDir string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode parsed artifact: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stepDir, "parsed.json"), append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write parsed artifact: %w", err)
	}
	return nil
}
