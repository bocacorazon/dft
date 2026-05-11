package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	dfterrors "github.com/bocacorazon/dft/internal/errors"
	"github.com/bocacorazon/dft/internal/functions"
	"github.com/bocacorazon/dft/internal/gitx"
	"github.com/bocacorazon/dft/internal/store"
	"github.com/bocacorazon/dft/internal/verify"
)

type AgentAdapter interface {
	RunAgent(ctx context.Context, workDir string, agent string, prompt string) (string, error)
}

type Engine struct {
	store   store.RunStore
	adapter AgentAdapter
}

type FlowDefinition struct {
	Steps  []FlowStep  `yaml:"steps"`
	Stages []FlowStage `yaml:"stages"`
}

type FlowStage struct {
	Name   string     `yaml:"name"`
	Before []FlowStep `yaml:"before"`
	Steps  []FlowStep `yaml:"steps"`
	After  []FlowStep `yaml:"after"`
}

type FlowStep struct {
	ID         string            `yaml:"id"`
	Type       string            `yaml:"type"`
	Agent      string            `yaml:"agent"`
	Fn         string            `yaml:"fn"`
	Args       map[string]string `yaml:"args"`
	Prompt     string            `yaml:"prompt"`
	Capture    bool              `yaml:"capture"`
	ExportAs   string            `yaml:"export_as"`
	RemoteOnly bool              `yaml:"remote_only"`
	Verify     StepVerifier      `yaml:"verify"`
}

type StepVerifier struct {
	Checks []verify.Check `yaml:"checks"`
}

type PlannedStep struct {
	ID   string
	Step FlowStep
}

func NewEngine(s store.RunStore, a AgentAdapter) *Engine {
	return &Engine{
		store:   s,
		adapter: a,
	}
}

func (e *Engine) Submit(ctx context.Context, flowFile string) (string, error) {
	flow, err := parseFlowFile(flowFile)
	if err != nil {
		return "", err
	}
	planned, err := flattenFlow(flow)
	if err != nil {
		return "", err
	}

	runID := newRunID()
	repoRoot, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	worktree, err := gitx.CreateWorktree(repoRoot, runID)
	if err != nil {
		return "", fmt.Errorf("create worktree: %w", err)
	}

	if err := e.store.StartRun(runID, flowFile); err != nil {
		return "", err
	}

	if err := e.executeRemaining(ctx, runID, worktree.Path, planned, 0, map[string]string{}); err != nil {
		_ = e.store.MarkRunFailed(runID, err.Error())
		return runID, err
	}

	if err := e.store.MarkRunSucceeded(runID); err != nil {
		return runID, err
	}

	if err := gitx.MergeAndCleanup(repoRoot, worktree); err != nil {
		_ = e.store.MarkRunFailed(runID, err.Error())
		return runID, err
	}

	return runID, nil
}

func (e *Engine) Resume(ctx context.Context, runID string) (string, error) {
	record, err := e.store.GetRun(runID)
	if err != nil {
		return "", err
	}

	flow, err := parseFlowFile(record.FlowFile)
	if err != nil {
		return runID, err
	}
	planned, err := flattenFlow(flow)
	if err != nil {
		return runID, err
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return runID, fmt.Errorf("get working directory: %w", err)
	}
	worktree, err := gitx.ExistingWorktree(repoRoot, runID)
	if err != nil {
		return runID, err
	}

	if err := e.store.MarkRunRunning(runID); err != nil {
		return runID, err
	}

	completed, exports, err := e.reconcileCompletedSteps(worktree.Path, runID, planned)
	if err != nil {
		_ = e.store.MarkRunFailed(runID, err.Error())
		return runID, err
	}

	start := 0
	for start < len(planned) && completed[planned[start].ID] {
		start++
	}

	if err := e.executeRemaining(ctx, runID, worktree.Path, planned, start, exports); err != nil {
		_ = e.store.MarkRunFailed(runID, err.Error())
		return runID, err
	}

	if err := e.store.MarkRunSucceeded(runID); err != nil {
		return runID, err
	}

	if err := gitx.MergeAndCleanup(repoRoot, worktree); err != nil {
		_ = e.store.MarkRunFailed(runID, err.Error())
		return runID, err
	}

	return runID, nil
}

func (e *Engine) executeRemaining(ctx context.Context, runID string, worktreePath string, planned []PlannedStep, start int, exports map[string]string) error {
	for i := start; i < len(planned); i++ {
		if err := e.executeStep(ctx, runID, worktreePath, planned[i], exports); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) executeStep(ctx context.Context, runID string, worktreePath string, planned PlannedStep, exports map[string]string) error {
	stepID := planned.ID
	step := planned.Step

	var stdout string
	switch step.Type {
	case "agent":
		if step.Agent == "" {
			return dfterrors.InvalidFlowError{Message: fmt.Sprintf("step %q missing agent", stepID)}
		}
		prompt := renderPrompt(step.Prompt, exports)
		out, err := e.adapter.RunAgent(ctx, worktreePath, step.Agent, prompt)
		if err != nil {
			return err
		}
		stdout = out
	case "function":
		renderedArgs := renderArgs(step.Args, exports)
		out, updates, err := functions.Execute(worktreePath, step.Fn, renderedArgs)
		if err != nil {
			return err
		}
		for k, v := range updates {
			exports[k] = v
		}
		stdout = out
	default:
		return dfterrors.UnsupportedStepTypeError{
			StepID:   stepID,
			StepType: step.Type,
		}
	}

	if err := e.store.WriteStepOutput(runID, stepID, stdout, step.Capture, step.ExportAs); err != nil {
		return err
	}

	if step.ExportAs != "" {
		exports[step.ExportAs] = strings.TrimSpace(stdout)
	}

	if !step.RemoteOnly {
		if _, err := gitx.CommitIfDirty(worktreePath, runID, stepID); err != nil {
			return err
		}
	}

	if len(step.Verify.Checks) > 0 {
		failures, err := verify.Evaluate(worktreePath, stepID, step.Verify.Checks)
		if err != nil {
			return err
		}
		if len(failures) > 0 {
			_ = e.store.WriteVerifyFailures(runID, failures)
			return fmt.Errorf("verify checks failed for step %s", stepID)
		}
	}

	return nil
}

func (e *Engine) reconcileCompletedSteps(worktreePath string, runID string, planned []PlannedStep) (map[string]bool, map[string]string, error) {
	committed, err := gitx.CommittedStepIDs(worktreePath, runID)
	if err != nil {
		return nil, nil, err
	}

	completed := make(map[string]bool, len(planned))
	exports := make(map[string]string)

	for _, p := range planned {
		hasArtifact, err := e.store.HasStepArtifact(runID, p.ID)
		if err != nil {
			return nil, nil, err
		}

		switch {
		case p.Step.RemoteOnly && hasArtifact:
			completed[p.ID] = true
		case committed[p.ID]:
			completed[p.ID] = true
		case hasArtifact:
			// Steps with no local mutation may not generate a commit.
			completed[p.ID] = true
		default:
			completed[p.ID] = false
		}

		if !completed[p.ID] {
			continue
		}

		if p.Step.Type == "function" && p.Step.Fn == "set_var" {
			renderedArgs := renderArgs(p.Step.Args, exports)
			name := strings.TrimSpace(renderedArgs["name"])
			if name != "" {
				exports[name] = renderedArgs["value"]
			}
		}

		if p.Step.ExportAs != "" {
			stdout, err := e.store.ReadStepStdout(runID, p.ID)
			if err != nil {
				return nil, nil, err
			}
			exports[p.Step.ExportAs] = strings.TrimSpace(stdout)
		}
	}

	return completed, exports, nil
}

func parseFlowFile(flowFile string) (FlowDefinition, error) {
	data, err := os.ReadFile(flowFile)
	if err != nil {
		return FlowDefinition{}, fmt.Errorf("read flow file: %w", err)
	}

	var flow FlowDefinition
	if err := yaml.Unmarshal(data, &flow); err != nil {
		return FlowDefinition{}, dfterrors.InvalidFlowError{Message: err.Error()}
	}
	if len(flow.Steps) == 0 && len(flow.Stages) == 0 {
		return FlowDefinition{}, dfterrors.InvalidFlowError{Message: "no steps defined"}
	}

	return flow, nil
}

func flattenFlow(flow FlowDefinition) ([]PlannedStep, error) {
	planned := make([]PlannedStep, 0)
	seen := make(map[string]bool)
	counter := 0
	add := func(step FlowStep) error {
		counter++
		id := strings.TrimSpace(step.ID)
		if id == "" {
			id = fmt.Sprintf("step-%d", counter)
		}
		if seen[id] {
			return dfterrors.InvalidFlowError{Message: fmt.Sprintf("duplicate step id: %s", id)}
		}
		seen[id] = true
		planned = append(planned, PlannedStep{ID: id, Step: step})
		return nil
	}

	if len(flow.Stages) > 0 {
		for _, stage := range flow.Stages {
			for _, step := range stage.Before {
				if err := add(step); err != nil {
					return nil, err
				}
			}
			for _, step := range stage.Steps {
				if err := add(step); err != nil {
					return nil, err
				}
			}
			for _, step := range stage.After {
				if err := add(step); err != nil {
					return nil, err
				}
			}
		}
		return planned, nil
	}

	for _, step := range flow.Steps {
		if err := add(step); err != nil {
			return nil, err
		}
	}

	return planned, nil
}

func renderPrompt(prompt string, exports map[string]string) string {
	rendered := prompt
	for key, value := range exports {
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
		rendered = strings.ReplaceAll(rendered, "{{ "+key+" }}", value)
	}
	return rendered
}

func renderArgs(args map[string]string, exports map[string]string) map[string]string {
	rendered := make(map[string]string, len(args))
	for k, v := range args {
		rendered[k] = renderPrompt(v, exports)
	}
	return rendered
}

func newRunID() string {
	ts := time.Now().UTC().Format("20060102-150405.000000000")
	sanitized := strings.ReplaceAll(ts, ".", "")
	return fmt.Sprintf("run-%s", sanitized)
}
