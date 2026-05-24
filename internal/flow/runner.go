package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/adapters/github"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/agentjson"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// StepType names the kind of work a typed flow step performs.
type StepType string

const (
	StepCommand  StepType = "command"
	StepAgent    StepType = "agent"
	StepGate     StepType = "gate"
	StepFunction StepType = "function"
	StepTool     StepType = "tool"
	StepVerify   StepType = "verify"
	StepWorkflow StepType = "workflow"
	StepLoop     StepType = "loop"
)

type AgentOutputMode string

const (
	AgentOutputJSON AgentOutputMode = "json"
	AgentOutputText AgentOutputMode = "text"
)

// StepStatus captures the terminal state of an executed step.
type StepStatus string

const (
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
	StepPaused    StepStatus = "paused"
)

// StepObserver receives rendered step lifecycle events from the runner.
type StepObserver interface {
	StepStarted(runID string, step Step, result Result)
	StepCompleted(runID string, step Step, status StepStatus, result Result)
}

// Definition is the minimal built-in flow shape used before external DSL support.
type Definition struct {
	MaxSpecParallelism int     `json:"max_spec_parallelism,omitempty"`
	Steps              []Step  `json:"steps"`
	Stages             []Stage `json:"stages,omitempty"`
}

// Stage groups setup, main, after, and verification work.
type Stage struct {
	ID     string         `json:"id"`
	Setup  []Step         `json:"setup,omitempty"`
	Steps  []Step         `json:"steps"`
	After  []Step         `json:"after,omitempty"`
	Verify []domain.Check `json:"verify,omitempty"`
}

// Step describes one typed flow step.
type Step struct {
	ID            string            `json:"id"`
	Type          StepType          `json:"type"`
	CommandName   string            `json:"command_name,omitempty"`
	CommandInput  string            `json:"command_input,omitempty"`
	Integration   string            `json:"integration,omitempty"`
	Model         string            `json:"model,omitempty"`
	AgentName     string            `json:"agent_name,omitempty"`
	OutputMode    AgentOutputMode   `json:"output_mode,omitempty"`
	AllowTools    bool              `json:"allow_tools,omitempty"`
	Prompt        string            `json:"prompt,omitempty"`
	Demand        string            `json:"demand,omitempty"`
	Cwd           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Function      string            `json:"function,omitempty"`
	Args          map[string]string `json:"args,omitempty"`
	MaxIterations int               `json:"max_iterations,omitempty"`
	NoContext     bool              `json:"no_context,omitempty"`
	Message       string            `json:"message,omitempty"`
	Setup         []Step            `json:"setup,omitempty"`
	Verify        []domain.Check    `json:"verify,omitempty"`
	Checks        []domain.Check    `json:"checks,omitempty"`
	OnError       string            `json:"on_error,omitempty"`
	Workflow      string            `json:"workflow,omitempty"`
	Steps         []Step            `json:"steps,omitempty"`
	ExitWhen      map[string]string `json:"exit_when,omitempty"`
	When          string            `json:"when,omitempty"`
}

// Runner executes typed flow definitions and writes per-step audit artifacts.
type Runner struct {
	Agent            ports.AgentAdapter
	Dispatcher       ports.CommandDispatcher
	ArtifactRoot     string
	RunID            string
	Verifier         ports.Verifier
	CommitLocalSteps bool
	AutoApproveGates bool
	Inputs           map[string]any
	Observer         StepObserver
}

// Result contains terminal status for every completed step.
type Result struct {
	Steps        []StepResult
	Inputs       map[string]any
	Vars         map[string]string
	StepOutputs  map[string]map[string]any
	Verification []domain.VerificationResult
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

	result := Result{
		Steps:       make([]StepResult, 0, len(definition.Steps)),
		Inputs:      cloneAnyMap(r.Inputs),
		Vars:        map[string]string{},
		StepOutputs: map[string]map[string]any{},
	}
	if _, ok := result.Inputs["run_id"]; !ok {
		result.Inputs["run_id"] = r.RunID
	}
	if _, ok := result.Inputs["artifact_root"]; !ok {
		result.Inputs["artifact_root"] = r.ArtifactRoot
	}
	if err := r.saveState(stateFromResult(result, 0, "running")); err != nil {
		return result, err
	}
	if len(definition.Stages) > 0 {
		result, err := r.executeStages(ctx, definition.Stages, result)
		if err != nil {
			_ = r.saveState(stateFromResult(result, 0, "failed"))
			return result, err
		}
		if err := r.saveState(stateFromResult(result, len(definition.Steps), "completed")); err != nil {
			return result, err
		}
		return result, nil
	}
	for i, step := range definition.Steps {
		stepResults, err := r.executeStepWithPolicy(ctx, step, &result)
		result.Steps = append(result.Steps, stepResults...)
		if err != nil {
			status := "failed"
			if isPauseError(err) {
				status = "paused"
			}
			if saveErr := r.saveState(stateFromResult(result, i, status)); saveErr != nil {
				return result, saveErr
			}
			return result, err
		}
		status := "running"
		if i+1 == len(definition.Steps) {
			status = "completed"
		}
		if err := r.saveState(stateFromResult(result, i+1, status)); err != nil {
			return result, err
		}
		if r.CommitLocalSteps && mutatesLocal(step) {
			if _, err := commitStep(ctx, commitRoot(r.ArtifactRoot, step), r.RunID, step.ID); err != nil {
				return result, err
			}
		}
	}
	if len(definition.Steps) == 0 {
		if err := r.saveState(stateFromResult(result, 0, "completed")); err != nil {
			return result, err
		}
	}
	return result, nil
}

// Resume continues execution from the saved top-level step index.
func (r Runner) Resume(ctx context.Context, definition Definition) (Result, error) {
	if r.RunID == "" {
		return Result{}, fmt.Errorf("run id is required")
	}
	if len(definition.Stages) > 0 {
		return Result{}, fmt.Errorf("resume is not supported for staged workflows")
	}
	state, err := r.loadState()
	if err != nil {
		return Result{}, err
	}
	return r.resumeFromState(ctx, definition, state, state.CurrentStepIndex)
}

// ResumeFrom continues execution from an explicit top-level step index using saved state.
func (r Runner) ResumeFrom(ctx context.Context, definition Definition, stepIndex int) (Result, error) {
	if r.RunID == "" {
		return Result{}, fmt.Errorf("run id is required")
	}
	if len(definition.Stages) > 0 {
		return Result{}, fmt.Errorf("resume is not supported for staged workflows")
	}
	state, err := r.loadState()
	if err != nil {
		return Result{}, err
	}
	return r.resumeFromState(ctx, definition, state, stepIndex)
}

func (r Runner) resumeFromState(ctx context.Context, definition Definition, state runState, stepIndex int) (Result, error) {
	result := resultFromState(state)
	result.Steps = make([]StepResult, 0, len(definition.Steps))
	if stepIndex < 0 {
		stepIndex = 0
	}
	if stepIndex > len(definition.Steps) {
		stepIndex = len(definition.Steps)
	}
	for i := stepIndex; i < len(definition.Steps); i++ {
		stepResults, stepErr := r.executeStepWithPolicy(ctx, definition.Steps[i], &result)
		result.Steps = append(result.Steps, stepResults...)
		if stepErr != nil {
			status := "failed"
			if isPauseError(stepErr) {
				status = "paused"
			}
			if saveErr := r.saveState(stateFromResult(result, i, status)); saveErr != nil {
				return result, saveErr
			}
			return result, stepErr
		}
		status := "running"
		if i+1 == len(definition.Steps) {
			status = "completed"
		}
		if err := r.saveState(stateFromResult(result, i+1, status)); err != nil {
			return result, err
		}
		if r.CommitLocalSteps && mutatesLocal(definition.Steps[i]) {
			if _, err := commitStep(ctx, commitRoot(r.ArtifactRoot, definition.Steps[i]), r.RunID, definition.Steps[i].ID); err != nil {
				return result, err
			}
		}
	}
	if len(definition.Steps) == 0 {
		if err := r.saveState(stateFromResult(result, 0, "completed")); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r Runner) executeStages(ctx context.Context, stages []Stage, result Result) (Result, error) {
	for _, stage := range stages {
		for _, step := range stage.Setup {
			stepResults, err := r.executeStepWithPolicy(ctx, step, &result)
			result.Steps = append(result.Steps, stepResults...)
			if err != nil {
				return result, fmt.Errorf("stage %q setup: %w", stage.ID, err)
			}
		}
		for _, step := range stage.Steps {
			stepResults, err := r.executeStepWithPolicy(ctx, step, &result)
			result.Steps = append(result.Steps, stepResults...)
			if err != nil {
				return result, fmt.Errorf("stage %q step: %w", stage.ID, err)
			}
		}
		for _, step := range stage.After {
			stepResults, err := r.executeStepWithPolicy(ctx, step, &result)
			result.Steps = append(result.Steps, stepResults...)
			if err != nil {
				return result, fmt.Errorf("stage %q after: %w", stage.ID, err)
			}
		}
		if len(stage.Verify) > 0 {
			if r.Verifier == nil {
				return result, fmt.Errorf("stage %q verifier is required", stage.ID)
			}
			verification := r.Verifier.Run(ctx, stage.Verify)
			result.Verification = append(result.Verification, verification)
			if verification.Status != domain.VerdictPass {
				return result, fmt.Errorf("stage %q verification failed", stage.ID)
			}
		}
	}
	return result, nil
}

func (r Runner) executeStepWithPolicy(ctx context.Context, step Step, result *Result) ([]StepResult, error) {
	var stepResults []StepResult
	for _, setup := range step.Setup {
		results, err := r.executeStepWithPolicy(ctx, setup, result)
		stepResults = append(stepResults, results...)
		if err != nil {
			return stepResults, fmt.Errorf("step %q setup: %w", step.ID, err)
		}
	}

	attempts := retryAttempts(step)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		rendered := renderStep(step, result)
		if r.Observer != nil {
			r.Observer.StepStarted(r.RunID, rendered, *result)
		}
		stepResult, err := r.executeStep(ctx, rendered, result)
		if r.Observer != nil {
			r.Observer.StepCompleted(r.RunID, rendered, stepResult.Status, *result)
		}
		stepResults = append(stepResults, stepResult)
		if err == nil {
			return stepResults, nil
		}
		if isPauseError(err) {
			return stepResults, err
		}
		lastErr = err
	}

	switch onErrorMode(step.OnError) {
	case "continue":
		if step.Type == StepLoop {
			return stepResults, lastErr
		}
		return stepResults, nil
	case "escalate":
		if err := writeInboxItem(r.ArtifactRoot, r.RunID, step.ID, map[string]string{
			"status":  "escalated",
			"message": lastErr.Error(),
		}); err != nil {
			return stepResults, err
		}
		return stepResults, lastErr
	default:
		return stepResults, lastErr
	}
}

func (r Runner) executeStep(ctx context.Context, step Step, result *Result) (StepResult, error) {
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
	if stepEnabled(step.When) == false {
		output := map[string]any{"status": "skipped", "when": step.When}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		if err := writeParsed(stepDir, output); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
		return StepResult{ID: step.ID, Type: step.Type, Status: StepSucceeded}, nil
	}

	switch step.Type {
	case StepCommand:
		if err := r.executeCommandStep(ctx, step, stepDir, result); err != nil {
			if isPauseError(err) {
				return StepResult{ID: step.ID, Type: step.Type, Status: StepPaused}, err
			}
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	case StepAgent:
		if err := r.executeAgentStep(ctx, step, stepDir, result); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	case StepGate:
		if err := r.executeGateStep(step, stepDir, result); err != nil {
			if isPauseError(err) {
				return StepResult{ID: step.ID, Type: step.Type, Status: StepPaused}, err
			}
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	case StepTool:
		if len(step.Command) == 0 {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("step %q command is required", step.ID)
		}
		cmd := exec.CommandContext(ctx, step.Command[0], step.Command[1:]...)
		cmd.Dir = step.Cwd
		if cmd.Dir == "" {
			cmd.Dir = r.ArtifactRoot
		}
		cmd.Env = os.Environ()
		for key, value := range step.Env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
		output, err := cmd.CombinedOutput()
		if writeErr := os.WriteFile(filepath.Join(stepDir, "stdout.txt"), output, 0o644); writeErr != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("write tool output artifact: %w", writeErr)
		}
		if err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("run tool step %q: %w", step.ID, err)
		}
		if err := writeParsed(stepDir, map[string]string{"status": "succeeded"}); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	case StepFunction:
		if err := r.executeFunctionStep(ctx, step, stepDir, result); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	case StepVerify:
		if err := r.executeVerifyStep(ctx, step, stepDir, result); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	case StepWorkflow:
		path := step.Workflow
		if path == "" {
			path = step.Args["path"]
		}
		if path == "" {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("workflow step %q requires path", step.ID)
		}
		definition, err := LoadDefinition(r.path(path))
		if err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
		workflowResult, err := r.Execute(ctx, definition)
		result.Steps = append(result.Steps, workflowResult.Steps...)
		result.Verification = append(result.Verification, workflowResult.Verification...)
		for key, value := range workflowResult.Vars {
			result.Vars[key] = value
		}
		if err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
		if err := writeParsed(stepDir, map[string]string{"status": "succeeded", "workflow": path}); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	case StepLoop:
		if err := r.executeLoopStep(ctx, step, stepDir, result); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	default:
		return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("unsupported step type %q", step.Type)
	}
	if err := r.verifyStep(ctx, step, result); err != nil {
		return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
	}
	return StepResult{ID: step.ID, Type: step.Type, Status: StepSucceeded}, nil
}

type pauseError struct {
	stepID string
}

func (e pauseError) Error() string {
	return fmt.Sprintf("workflow paused at gate %q", e.stepID)
}

func isPauseError(err error) bool {
	var target pauseError
	return errors.As(err, &target)
}

func (r Runner) executeCommandStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	if r.Dispatcher == nil {
		return fmt.Errorf("command dispatcher is required")
	}
	if step.CommandName == "" {
		return fmt.Errorf("step %q command name is required", step.ID)
	}
	input := step.CommandInput
	if !step.NoContext {
		contextualInput, hashes, err := attachProjectContext(r.ArtifactRoot, input)
		if err != nil {
			return err
		}
		input = contextualInput
		if err := writeContextHashes(stepDir, hashes); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(stepDir, "input.txt"), []byte(input), 0o644); err != nil {
		return fmt.Errorf("write command input artifact: %w", err)
	}
	response, err := r.Dispatcher.DispatchCommand(ctx, ports.CommandRequest{
		Command:     step.CommandName,
		Input:       input,
		RunID:       r.RunID,
		Cwd:         step.Cwd,
		Env:         step.Env,
		Integration: step.Integration,
		Model:       step.Model,
		AllowTools:  step.AllowTools,
	})
	if err != nil {
		return fmt.Errorf("dispatch command step %q: %w", step.ID, err)
	}
	if err := os.WriteFile(filepath.Join(stepDir, "stdout.txt"), []byte(response.Stdout), 0o644); err != nil {
		return fmt.Errorf("write command stdout artifact: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stepDir, "stderr.txt"), []byte(response.Stderr), 0o644); err != nil {
		return fmt.Errorf("write command stderr artifact: %w", err)
	}
	output := map[string]any{
		"command":     step.CommandName,
		"input":       input,
		"integration": step.Integration,
		"model":       step.Model,
		"stdout":      response.Stdout,
		"stderr":      response.Stderr,
		"exit_code":   response.ExitCode,
	}
	artifactInfo, artifactErr := verifySpeckitCommandArtifacts(r.ArtifactRoot, step)
	if artifactInfo != nil {
		output["artifacts"] = artifactInfo
	}
	if artifactErr != nil {
		output["artifact_error"] = artifactErr.Error()
	}
	if step.CommandName == "speckit.analyze" {
		analysis, err := parseAnalyzeOutput(response.Stdout)
		if err != nil {
			return fmt.Errorf("parse speckit.analyze output: %w", err)
		}
		for key, value := range analysis {
			output[key] = value
		}
	}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	if err := writeParsed(stepDir, output); err != nil {
		return err
	}
	if response.ExitCode != 0 {
		message := strings.TrimSpace(response.Stderr)
		if message == "" {
			message = fmt.Sprintf("command exited with code %d", response.ExitCode)
		}
		return fmt.Errorf("%s", message)
	}
	if artifactErr != nil {
		return artifactErr
	}
	return nil
}

func (r Runner) runVerification(ctx context.Context, step Step, checks []domain.Check) (domain.VerificationResult, error) {
	if r.Verifier == nil {
		return domain.VerificationResult{}, fmt.Errorf("step %q verifier is required", step.ID)
	}
	switch verifier := r.Verifier.(type) {
	case verify.Checker:
		if step.Cwd == "" {
			return verifier.Run(ctx, checks), nil
		}
		verifier.RootDir = step.Cwd
		return verifier.Run(ctx, checks), nil
	case *verify.Checker:
		if verifier == nil {
			return domain.VerificationResult{}, fmt.Errorf("step %q verifier is required", step.ID)
		}
		copy := *verifier
		if step.Cwd != "" {
			copy.RootDir = step.Cwd
		}
		return copy.Run(ctx, checks), nil
	default:
		return r.Verifier.Run(ctx, checks), nil
	}
}

func (r Runner) executeGateStep(step Step, stepDir string, result *Result) error {
	output := map[string]any{
		"message": step.Message,
	}
	if r.AutoApproveGates {
		output["choice"] = "approve"
		output["status"] = "approved"
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	output["status"] = "paused"
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	if err := writeInboxItem(r.ArtifactRoot, r.RunID, step.ID, output); err != nil {
		return err
	}
	if err := writeParsed(stepDir, output); err != nil {
		return err
	}
	return pauseError{stepID: step.ID}
}

func (r Runner) executeFunctionStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	root := r.ArtifactRoot
	if step.Cwd != "" {
		root = step.Cwd
	}
	switch step.Function {
	case "set_var":
		name := step.Args["name"]
		value := step.Args["value"]
		if name == "" {
			return fmt.Errorf("set_var requires name")
		}
		result.Vars[name] = value
		output := map[string]any{name: value}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "gh_pr_create":
		record, err := githubAdapter(root, step).CreatePR(ctx, github.PRRequest{
			RunID:  r.RunID,
			StepID: step.ID,
			Head:   step.Args["head"],
			Base:   step.Args["base"],
			Title:  step.Args["title"],
		})
		if err != nil {
			return err
		}
		if record.Number != 0 {
			result.Vars["pr_number"] = strconv.Itoa(record.Number)
		}
		result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
			"number":      record.Number,
			"status":      record.Status,
			"remote_only": record.RemoteOnly,
			"output":      record.Output,
		})
		return writeParsed(stepDir, record)
	case "gh_pr_number_for_branch":
		record, err := githubAdapter(root, step).PRNumberForBranch(ctx, github.BranchPRRequest{
			RunID:  r.RunID,
			StepID: step.ID,
			Head:   step.Args["head"],
		})
		if err != nil {
			return err
		}
		if record.Number != 0 {
			result.Vars["pr_number"] = strconv.Itoa(record.Number)
		}
		result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
			"number":      record.Number,
			"status":      record.Status,
			"remote_only": record.RemoteOnly,
			"output":      record.Output,
		})
		return writeParsed(stepDir, record)
	case "gh_pr_wait_checks":
		number, err := stepPRNumber(step, result)
		if err != nil {
			return err
		}
		record, err := githubAdapter(root, step).WaitChecks(ctx, github.CheckRequest{RunID: r.RunID, StepID: step.ID, Number: number})
		if err != nil {
			return err
		}
		result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
			"number":      record.Number,
			"status":      record.Status,
			"remote_only": record.RemoteOnly,
			"output":      record.Output,
		})
		return writeParsed(stepDir, record)
	case "gh_pr_merge":
		number, err := stepPRNumber(step, result)
		if err != nil {
			return err
		}
		record, err := githubAdapter(root, step).MergePR(ctx, github.MergeRequest{RunID: r.RunID, StepID: step.ID, Number: number, Method: step.Args["method"]})
		if err != nil {
			return err
		}
		result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
			"number":      record.Number,
			"status":      record.Status,
			"remote_only": record.RemoteOnly,
			"output":      record.Output,
		})
		return writeParsed(stepDir, record)
	case "wait_for_human":
		record := map[string]string{
			"status":  "blocked",
			"message": step.Args["message"],
		}
		if err := writeInboxItem(r.ArtifactRoot, r.RunID, step.ID, record); err != nil {
			return err
		}
		result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{"status": "blocked", "message": step.Args["message"]})
		if err := writeParsed(stepDir, record); err != nil {
			return err
		}
		return fmt.Errorf("wait_for_human blocked run")
	case "commit_message":
		message := strings.TrimSpace(step.Args["title"] + "\n\n" + step.Args["body"])
		if message == "" {
			return fmt.Errorf("commit_message requires title or body")
		}
		result.Vars["commit_message"] = message
		output := map[string]any{"commit_message": message}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "git_branch_current":
		if isGitRepo(ctx, root) {
			branch, err := currentGitBranch(ctx, root)
			if err != nil {
				return err
			}
			result.Vars["current_branch"] = branch
			output := map[string]any{"current_branch": branch}
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		if branch := strings.TrimSpace(step.Env["GIT_BRANCH_NAME"]); branch != "" {
			result.Vars["current_branch"] = branch
			output := map[string]any{"current_branch": branch}
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		output := map[string]any{"current_branch": ""}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "git_checkout_branch":
		branch := strings.TrimSpace(step.Args["branch"])
		if branch == "" {
			branch = strings.TrimSpace(step.Env["GIT_BRANCH_NAME"])
		}
		if branch == "" {
			return fmt.Errorf("git_checkout_branch requires branch")
		}
		output := map[string]any{"branch": branch}
		if !isGitRepo(ctx, root) {
			output["status"] = "noop"
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		if _, err := runGit(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			if _, err := runGit(ctx, root, "switch", branch); err != nil {
				return err
			}
			output["status"] = "switched"
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		if _, err := runGit(ctx, root, "switch", "-c", branch); err != nil {
			return err
		}
		output["status"] = "created"
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "git_default_branch":
		branch, err := defaultBranch(ctx, root)
		if err != nil {
			return err
		}
		result.Vars["default_branch"] = branch
		output := map[string]any{"default_branch": branch}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "branch":
		name := step.Args["name"]
		base := step.Args["base"]
		if name == "" {
			return fmt.Errorf("branch requires name")
		}
		args := []string{"switch", "-c", name}
		if base != "" {
			args = append(args, base)
		}
		if _, err := runGit(ctx, root, args...); err != nil {
			return err
		}
		output := map[string]any{"branch": name, "base": base}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "merge":
		source := step.Args["source"]
		target := step.Args["target"]
		if source == "" || target == "" {
			return fmt.Errorf("merge requires source and target")
		}
		if _, err := runGit(ctx, root, "switch", target); err != nil {
			return err
		}
		if _, err := runGit(ctx, root, "merge", "--no-ff", "--no-edit", source); err != nil {
			return err
		}
		output := map[string]any{"source": source, "target": target, "status": "merged"}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "git_push":
		remote := step.Args["remote"]
		branch := step.Args["branch"]
		if remote == "" {
			remote = "origin"
		}
		if branch == "" {
			return fmt.Errorf("git_push requires branch")
		}
		record := map[string]string{"remote": remote, "branch": branch, "remote_only": "true"}
		if step.Args["dry_run"] != "false" {
			if err := writeRemoteAudit(root, r.RunID, step.ID, record); err != nil {
				return err
			}
			result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{"remote": remote, "branch": branch, "remote_only": true})
			return writeParsed(stepDir, record)
		}
		if _, err := runGit(ctx, root, "push", remote, branch); err != nil {
			return err
		}
		record["status"] = "pushed"
		if err := writeRemoteAudit(root, r.RunID, step.ID, record); err != nil {
			return err
		}
		result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{"remote": remote, "branch": branch, "remote_only": true, "status": "pushed"})
		return writeParsed(stepDir, record)
	case "git_commit_all":
		message := strings.TrimSpace(step.Args["message"])
		if message == "" {
			message = strings.TrimSpace(step.Args["title"] + "\n\n" + step.Args["body"])
		}
		if message == "" {
			return fmt.Errorf("git_commit_all requires message, title, or body")
		}
		status, err := runGit(ctx, root, "status", "--porcelain")
		if err != nil {
			return err
		}
		output := map[string]any{"message": message}
		if strings.TrimSpace(status) == "" {
			output["status"] = "noop"
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		if _, err := runGit(ctx, root, "add", "-A"); err != nil {
			return err
		}
		if _, err := runGit(ctx, root, "commit", "-m", message); err != nil {
			return err
		}
		commit, err := runGit(ctx, root, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		output["status"] = "committed"
		output["commit"] = strings.TrimSpace(commit)
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "gh_issues_from_findings":
		sourceStep := step.Args["step"]
		if sourceStep == "" {
			return fmt.Errorf("gh_issues_from_findings requires step")
		}
		stepOutput, ok := result.StepOutputs[sourceStep]
		if !ok {
			return fmt.Errorf("gh_issues_from_findings step %q has no output", sourceStep)
		}
		rawFindings, ok := stepOutput["findings"].([]any)
		if !ok || len(rawFindings) == 0 {
			output := map[string]any{"source_step": sourceStep, "created": 0, "issues": []any{}}
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		issues := make([]any, 0, len(rawFindings))
		for i, finding := range rawFindings {
			record, err := githubAdapter(root, step).CreateIssue(ctx, github.IssueRequest{
				RunID:  r.RunID,
				StepID: fmt.Sprintf("%s-%d", step.ID, i+1),
				Title:  formatFindingIssueTitle(step.Args["title_prefix"], finding),
				Body:   formatFindingIssueBody(sourceStep, finding),
			})
			if err != nil {
				return err
			}
			issues = append(issues, map[string]any{
				"title":       record.Title,
				"status":      record.Status,
				"remote_only": record.RemoteOnly,
				"output":      record.Output,
			})
		}
		output := map[string]any{"source_step": sourceStep, "created": len(issues), "issues": issues}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "git_rebase_merge_back":
		source := step.Args["source"]
		target := step.Args["target"]
		if source == "" || target == "" {
			return fmt.Errorf("git_rebase_merge_back requires source and target")
		}
		output := map[string]any{"source": source, "target": target}
		if _, err := runGit(ctx, root, "switch", source); err != nil {
			return err
		}
		if _, err := runGit(ctx, root, "rebase", target); err != nil {
			conflicted, detectErr := hasUnmergedFiles(ctx, root)
			if detectErr != nil {
				return detectErr
			}
			if conflicted {
				output["status"] = "conflict"
				output["phase"] = "rebase"
				output["message"] = err.Error()
				result.StepOutputs[step.ID] = cloneAnyMap(output)
				return writeParsed(stepDir, output)
			}
			return err
		}
		output["status"] = "rebased"
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	case "git_finalize_squash_merge_back":
		source := step.Args["source"]
		target := step.Args["target"]
		if source == "" || target == "" {
			return fmt.Errorf("git_finalize_squash_merge_back requires source and target")
		}
		remote := strings.TrimSpace(step.Args["remote"])
		if remote == "" {
			remote = "origin"
		}
		message := strings.TrimSpace(step.Args["message"])
		if message == "" {
			message = fmt.Sprintf("chore: squash merge %s into %s", source, target)
		}
		output := map[string]any{
			"source": source,
			"target": target,
			"remote": remote,
		}
		if conflicted, err := hasUnmergedFiles(ctx, root); err != nil {
			return err
		} else if conflicted {
			return fmt.Errorf("cannot finalize mergeback with unresolved conflicts")
		}
		sourceExists, err := localBranchExists(ctx, root, source)
		if err != nil {
			return err
		}
		if !sourceExists {
			targetTree, err := revParseTree(ctx, root, target)
			if err != nil {
				return err
			}
			output["status"] = "already-finalized"
			output["source_tree"] = targetTree
			output["target_tree"] = targetTree
			output["trees_equal"] = true
			output["local_branch_deleted"] = true
			remoteExists, err := remoteBranchExists(ctx, root, remote, source)
			if err != nil {
				return err
			}
			output["remote_branch_existed"] = remoteExists
			if err := releaseBranchIfCurrent(ctx, root, target, output); err != nil {
				return err
			}
			if remoteExists {
				if _, err := runGit(ctx, root, "push", remote, "--delete", source); err != nil {
					return err
				}
			}
			output["remote_branch_deleted_or_missing"] = true
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		sourceTree, err := revParseTree(ctx, root, source)
		if err != nil {
			return err
		}
		if _, err := runGit(ctx, root, "switch", target); err != nil {
			return err
		}
		if _, err := runGit(ctx, root, "merge", "--squash", source); err != nil {
			return err
		}
		status, err := runGit(ctx, root, "status", "--porcelain", "--untracked-files=no")
		if err != nil {
			return err
		}
		if strings.TrimSpace(status) == "" {
			output["status"] = "already-squashed"
		} else {
			if _, err := runGit(ctx, root, "commit", "-m", message); err != nil {
				return err
			}
			output["status"] = "merged"
			commit, err := runGit(ctx, root, "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			output["commit"] = strings.TrimSpace(commit)
		}
		targetTree, err := revParseTree(ctx, root, "HEAD")
		if err != nil {
			return err
		}
		output["source_tree"] = sourceTree
		output["target_tree"] = targetTree
		output["trees_equal"] = sourceTree == targetTree
		if sourceTree != targetTree {
			return fmt.Errorf("squash merge result tree does not match source branch tree")
		}
		if _, err := runGit(ctx, root, "branch", "-D", source); err != nil {
			return err
		}
		output["local_branch_deleted"] = true
		if err := releaseBranchIfCurrent(ctx, root, target, output); err != nil {
			return err
		}
		remoteExists, err := remoteBranchExists(ctx, root, remote, source)
		if err != nil {
			return err
		}
		output["remote_branch_existed"] = remoteExists
		if remoteExists {
			if _, err := runGit(ctx, root, "push", remote, "--delete", source); err != nil {
				return err
			}
		}
		output["remote_branch_deleted_or_missing"] = true
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	default:
		return fmt.Errorf("unsupported function %q", step.Function)
	}
}

func (r Runner) executeVerifyStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	checks := step.Checks
	if len(checks) == 0 {
		checks = step.Verify
	}
	checks = renderChecks(checks, result)
	if len(checks) == 0 {
		return fmt.Errorf("verify step %q requires checks", step.ID)
	}
	verification, err := r.runVerification(ctx, step, checks)
	if err != nil {
		return err
	}
	result.Verification = append(result.Verification, verification)
	if err := writeParsed(stepDir, verification); err != nil {
		return err
	}
	if verification.Status != domain.VerdictPass {
		return fmt.Errorf("verify step %q failed", step.ID)
	}
	return nil
}

func (r Runner) verifyStep(ctx context.Context, step Step, result *Result) error {
	if len(step.Verify) == 0 || step.Type == StepVerify {
		return nil
	}
	verification, err := r.runVerification(ctx, step, renderChecks(step.Verify, result))
	if err != nil {
		return err
	}
	result.Verification = append(result.Verification, verification)
	if verification.Status != domain.VerdictPass {
		return fmt.Errorf("step %q verification failed", step.ID)
	}
	return nil
}

func (r Runner) executeLoopStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	if step.MaxIterations <= 0 {
		return fmt.Errorf("loop step %q requires max_iterations", step.ID)
	}
	if len(step.Steps) == 0 {
		return fmt.Errorf("loop step %q requires steps", step.ID)
	}
	for i := 0; i < step.MaxIterations; i++ {
		for _, nested := range step.Steps {
			stepResults, err := r.executeStepWithPolicy(ctx, nested, result)
			result.Steps = append(result.Steps, stepResults...)
			if err != nil {
				return fmt.Errorf("loop step %q iteration %d: %w", step.ID, i+1, err)
			}
		}
		if r.loopExit(step, result) {
			output := map[string]any{"status": "succeeded", "iterations": i + 1}
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
	}
	if len(step.ExitWhen) > 0 {
		if onErrorMode(step.OnError) == "continue" {
			output := map[string]any{"status": "exhausted", "iterations": step.MaxIterations}
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		return fmt.Errorf("loop step %q exhausted %d iteration(s)", step.ID, step.MaxIterations)
	}
	output := map[string]any{"status": "succeeded", "iterations": step.MaxIterations}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) loopExit(step Step, result *Result) bool {
	if len(step.ExitWhen) == 0 {
		return false
	}
	if path := step.ExitWhen["file_exists"]; path != "" {
		if _, err := os.Stat(r.path(renderString(path, result))); err == nil {
			return true
		}
	}
	if value := step.ExitWhen["no_critical_findings"]; value == "true" {
		if len(result.Verification) == 0 {
			return false
		}
		return result.Verification[len(result.Verification)-1].Status == domain.VerdictPass
	}
	if value := step.ExitWhen["check_passes"]; value != "" {
		for i := len(result.Verification) - 1; i >= 0; i-- {
			for _, check := range result.Verification[i].Results {
				if check.CheckID == value {
					return check.Passed
				}
			}
		}
	}
	if value := step.ExitWhen["step_output_equals"]; value != "" {
		left, expected, ok := strings.Cut(value, "=")
		if !ok {
			return false
		}
		left = strings.TrimSpace(left)
		expected = strings.TrimSpace(expected)
		stepID, path, ok := strings.Cut(left, ".")
		if !ok || stepID == "" || path == "" {
			return false
		}
		output, ok := result.StepOutputs[stepID]
		if !ok {
			return false
		}
		resolved, ok := lookupPath(output, strings.Split(path, "."))
		if !ok {
			return false
		}
		return fmt.Sprint(resolved) == expected
	}
	return false
}

func githubAdapter(root string, step Step) github.Adapter {
	return github.Adapter{
		RootDir: root,
		DryRun:  step.Args["dry_run"] != "false",
		Binary:  step.Args["binary"],
	}
}

func stepPRNumber(step Step, result *Result) (int, error) {
	value := step.Args["number"]
	if value == "" {
		value = result.Vars["pr_number"]
	}
	if value == "" {
		return 0, fmt.Errorf("%s requires PR number", step.Function)
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse PR number: %w", err)
	}
	return number, nil
}

func runGit(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func defaultBranch(ctx context.Context, root string) (string, error) {
	out, err := runGit(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(strings.TrimSpace(out), "origin/"), nil
	}
	out, err = runGit(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func revParseTree(ctx context.Context, root string, ref string) (string, error) {
	output, err := runGit(ctx, root, "rev-parse", ref+"^{tree}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func isGitRepo(ctx context.Context, root string) bool {
	_, err := runGit(ctx, root, "rev-parse", "--git-dir")
	return err == nil
}

func remoteBranchExists(ctx context.Context, root string, remote string, branch string) (bool, error) {
	if _, err := runGit(ctx, root, "remote", "get-url", remote); err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "no such remote") || strings.Contains(lower, "not a git repository") {
			return false, nil
		}
		return false, err
	}
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--exit-code", "--heads", remote, branch)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(output)) != "", nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 2 {
		return false, nil
	}
	return false, fmt.Errorf("git ls-remote --exit-code --heads %s %s failed: %w: %s", remote, branch, err, strings.TrimSpace(string(output)))
}

func localBranchExists(ctx context.Context, root string, branch string) (bool, error) {
	output, err := runGit(ctx, root, "branch", "--list", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func currentGitBranch(ctx context.Context, root string) (string, error) {
	output, err := runGit(ctx, root, "branch", "--show-current")
	if err == nil {
		return strings.TrimSpace(output), nil
	}
	output, err = runGit(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return normalizeBranchName(strings.TrimSpace(output)), nil
}

func normalizeBranchName(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	branch = strings.TrimPrefix(branch, "heads/")
	return branch
}

func releaseBranchIfCurrent(ctx context.Context, root string, branch string, output map[string]any) error {
	current, err := currentGitBranch(ctx, root)
	if err != nil {
		return err
	}
	if current != branch {
		output["target_branch_released"] = false
		return nil
	}
	if _, err := runGit(ctx, root, "switch", "--detach", "HEAD"); err != nil {
		return fmt.Errorf("release branch %q after mergeback: %w", branch, err)
	}
	output["target_branch_released"] = true
	return nil
}

func hasUnmergedFiles(ctx context.Context, root string) (bool, error) {
	output, err := runGit(ctx, root, "ls-files", "--unmerged")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func formatFindingIssueTitle(prefix string, finding any) string {
	details, ok := finding.(map[string]any)
	if !ok {
		if prefix == "" {
			prefix = "Follow-up"
		}
		return strings.TrimSpace(prefix + ": unresolved finding")
	}
	severity := normalizeSeverity(fmt.Sprint(details["severity"]))
	if severity == "" {
		severity = "HIGH"
	}
	message := strings.TrimSpace(fmt.Sprint(details["message"]))
	if message == "" {
		message = "unresolved finding"
	}
	if prefix == "" {
		prefix = "Follow-up"
	}
	return strings.TrimSpace(prefix + " [" + severity + "]: " + message)
}

func formatFindingIssueBody(sourceStep string, finding any) string {
	details, ok := finding.(map[string]any)
	if !ok {
		return "Created by dft from unresolved findings in step " + sourceStep + "."
	}
	var builder strings.Builder
	builder.WriteString("Created by dft from unresolved findings in step `")
	builder.WriteString(sourceStep)
	builder.WriteString("`.\n\n")
	if value := strings.TrimSpace(fmt.Sprint(details["finding_id"])); value != "" && value != "<nil>" {
		builder.WriteString("- Finding ID: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	if value := normalizeSeverity(fmt.Sprint(details["severity"])); value != "" {
		builder.WriteString("- Severity: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	if value := strings.TrimSpace(fmt.Sprint(details["category"])); value != "" && value != "<nil>" {
		builder.WriteString("- Category: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	if value := strings.TrimSpace(fmt.Sprint(details["location"])); value != "" && value != "<nil>" {
		builder.WriteString("- Location: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	builder.WriteString("\n## Summary\n\n")
	builder.WriteString(strings.TrimSpace(fmt.Sprint(details["message"])))
	if value := strings.TrimSpace(fmt.Sprint(details["recommendation"])); value != "" && value != "<nil>" {
		builder.WriteString("\n\n## Recommendation\n\n")
		builder.WriteString(value)
	}
	return builder.String()
}

func retryAttempts(step Step) int {
	if step.Type == StepLoop {
		return 1
	}
	if step.MaxIterations > 0 {
		return step.MaxIterations
	}
	mode := strings.TrimSpace(step.OnError)
	if strings.HasPrefix(mode, "retry(") && strings.HasSuffix(mode, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(mode, "retry("), ")")
		parts := strings.Split(inner, ",")
		count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err == nil && count > 0 {
			return count + 1
		}
	}
	return 1
}

func onErrorMode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "retry(") {
		return "fail"
	}
	return value
}

func renderStep(step Step, result *Result) Step {
	step.CommandName = renderString(step.CommandName, result)
	step.CommandInput = renderString(step.CommandInput, result)
	step.Integration = renderString(step.Integration, result)
	step.Model = renderString(step.Model, result)
	step.Prompt = renderString(step.Prompt, result)
	step.Demand = renderString(step.Demand, result)
	step.Cwd = renderString(step.Cwd, result)
	step.Workflow = renderString(step.Workflow, result)
	step.Message = renderString(step.Message, result)
	step.When = renderString(step.When, result)
	for i, value := range step.Command {
		step.Command[i] = renderString(value, result)
	}
	for key, value := range step.Env {
		step.Env[key] = renderString(value, result)
	}
	for key, value := range step.Args {
		step.Args[key] = renderString(value, result)
	}
	step.Verify = renderChecks(step.Verify, result)
	step.Checks = renderChecks(step.Checks, result)
	for key, value := range step.ExitWhen {
		step.ExitWhen[key] = renderString(value, result)
	}
	return step
}

func renderChecks(checks []domain.Check, result *Result) []domain.Check {
	if len(checks) == 0 {
		return nil
	}
	rendered := make([]domain.Check, len(checks))
	for i, check := range checks {
		rendered[i] = check
		if len(check.Args) == 0 {
			continue
		}
		rendered[i].Args = make([]string, len(check.Args))
		for j, arg := range check.Args {
			rendered[i].Args[j] = renderString(arg, result)
		}
	}
	return rendered
}

func stepEnabled(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off", "<nil>":
		return false
	default:
		return value != ""
	}
}

func renderString(value string, result *Result) string {
	if value == "" || result == nil {
		return value
	}
	for _, expr := range extractExpressions(value) {
		if resolved, ok := lookupExpression(expr, result); ok {
			value = strings.ReplaceAll(value, "{{ "+expr+" }}", resolved)
		}
	}
	for key, replacement := range result.Vars {
		value = strings.ReplaceAll(value, "{{ vars."+key+" }}", replacement)
		value = strings.ReplaceAll(value, "{{ "+key+" }}", replacement)
	}
	value = os.Expand(value, func(key string) string {
		if strings.HasPrefix(key, "vars.") {
			return result.Vars[strings.TrimPrefix(key, "vars.")]
		}
		if strings.HasPrefix(key, "env.") {
			return os.Getenv(strings.TrimPrefix(key, "env."))
		}
		return os.Getenv(key)
	})
	return value
}

func extractExpressions(value string) []string {
	matches := regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`).FindAllStringSubmatch(value, -1)
	seen := map[string]struct{}{}
	expressions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		expr := strings.TrimSpace(match[1])
		if _, ok := seen[expr]; ok {
			continue
		}
		seen[expr] = struct{}{}
		expressions = append(expressions, expr)
	}
	return expressions
}

func lookupExpression(expr string, result *Result) (string, bool) {
	if result == nil {
		return "", false
	}
	if strings.HasPrefix(expr, "inputs.") {
		value, ok := lookupPath(result.Inputs, strings.Split(strings.TrimPrefix(expr, "inputs."), "."))
		if ok {
			return fmt.Sprint(value), true
		}
		return "", false
	}
	if strings.HasPrefix(expr, "steps.") {
		path := strings.Split(strings.TrimPrefix(expr, "steps."), ".")
		if len(path) >= 3 && path[1] == "output" {
			stepOutput, ok := result.StepOutputs[path[0]]
			if !ok {
				return "", false
			}
			value, ok := lookupPath(stepOutput, path[2:])
			if ok {
				return fmt.Sprint(value), true
			}
		}
	}
	return "", false
}

func lookupPath(value any, path []string) (any, bool) {
	if len(path) == 0 {
		return value, true
	}
	current, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	next, ok := current[path[0]]
	if !ok {
		return nil, false
	}
	return lookupPath(next, path[1:])
}

func mutatesLocal(step Step) bool {
	if step.Type == StepTool || step.Type == StepAgent || step.Type == StepCommand {
		return true
	}
	if step.Type != StepFunction {
		return false
	}
	switch step.Function {
	case "branch", "merge":
		return true
	default:
		return false
	}
}

func commitRoot(root string, step Step) string {
	if step.Cwd != "" && (step.Type == StepAgent || step.Type == StepTool || step.Type == StepCommand) {
		return step.Cwd
	}
	return root
}

func commitStep(ctx context.Context, root string, runID string, stepID string) (string, error) {
	status, err := runGit(ctx, root, "status", "--porcelain")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return "", nil
		}
		return "", err
	}
	if strings.TrimSpace(status) == "" {
		return "", nil
	}
	if _, err := runGit(ctx, root, "add", "-A"); err != nil {
		return "", err
	}
	message := fmt.Sprintf("chore: commit dft step %s\n\nRun-ID: %s\nStep-ID: %s", stepID, runID, stepID)
	if _, err := runGit(ctx, root, "commit", "-m", message); err != nil {
		return "", err
	}
	commit, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(commit), nil
}

func writeInboxItem(root string, runID string, stepID string, value any) error {
	path := filepath.Join(root, ".dft", "inbox", runID+"-"+stepID+".json")
	return writeJSONArtifact(path, value)
}

func (r Runner) path(path string) string {
	if filepath.IsAbs(path) || r.ArtifactRoot == "" {
		return path
	}
	return filepath.Join(r.ArtifactRoot, path)
}

func writeRemoteAudit(root string, runID string, stepID string, value any) error {
	path := filepath.Join(root, ".dft", "runs", runID, "remote", stepID+".json")
	return writeJSONArtifact(path, value)
}

func writeJSONArtifact(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode artifact: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}
	return nil
}

func (r Runner) executeAgentStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	if r.Agent == nil {
		return fmt.Errorf("agent adapter is required")
	}
	if step.AgentName == "" {
		return fmt.Errorf("step %q agent name is required", step.ID)
	}
	prompt := step.Prompt
	if !step.NoContext {
		contextualPrompt, hashes, err := attachProjectContext(r.ArtifactRoot, prompt)
		if err != nil {
			return err
		}
		prompt = contextualPrompt
		if err := writeContextHashes(stepDir, hashes); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(stepDir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("write prompt artifact: %w", err)
	}
	request := ports.AgentRequest{
		AgentName:  step.AgentName,
		Prompt:     prompt,
		Demand:     step.Demand,
		RunID:      r.RunID,
		Cwd:        step.Cwd,
		Env:        step.Env,
		AllowTools: step.AllowTools,
	}
	response, err := r.Agent.Invoke(ctx, request)
	if err != nil {
		return fmt.Errorf("invoke agent step %q: %w", step.ID, err)
	}
	if step.OutputMode == "" || step.OutputMode == AgentOutputJSON {
		var parsed any
		finalRaw := response.Raw
		firstRaw := ""
		if err := agentjson.DecodeFirst(finalRaw, &parsed); err != nil {
			firstRaw = finalRaw
			retryRequest := request
			retryRequest.Prompt = prompt + "\n\nIMPORTANT: Return ONLY a single valid JSON value matching the required schema. Do not include any prose, markdown, code fences, headings, or explanations."
			retryResponse, retryErr := r.Agent.Invoke(ctx, retryRequest)
			if retryErr != nil {
				if writeErr := os.WriteFile(filepath.Join(stepDir, "stdout.txt"), []byte(finalRaw), 0o644); writeErr != nil {
					return fmt.Errorf("write stdout artifact: %w", writeErr)
				}
				return fmt.Errorf("parse agent step %q output: %w; retry invoke failed: %v", step.ID, err, retryErr)
			}
			finalRaw = retryResponse.Raw
			if retryParseErr := agentjson.DecodeFirst(finalRaw, &parsed); retryParseErr != nil {
				if writeErr := os.WriteFile(filepath.Join(stepDir, "stdout.txt"), []byte(finalRaw), 0o644); writeErr != nil {
					return fmt.Errorf("write stdout artifact: %w", writeErr)
				}
				if firstRaw != "" {
					if writeErr := os.WriteFile(filepath.Join(stepDir, "stdout-attempt-1.txt"), []byte(firstRaw), 0o644); writeErr != nil {
						return fmt.Errorf("write retry stdout artifact: %w", writeErr)
					}
				}
				return fmt.Errorf("parse agent step %q output: initial=%v retry=%v", step.ID, err, retryParseErr)
			}
		}
		if err := os.WriteFile(filepath.Join(stepDir, "stdout.txt"), []byte(finalRaw), 0o644); err != nil {
			return fmt.Errorf("write stdout artifact: %w", err)
		}
		if firstRaw != "" {
			if err := os.WriteFile(filepath.Join(stepDir, "stdout-attempt-1.txt"), []byte(firstRaw), 0o644); err != nil {
				return fmt.Errorf("write retry stdout artifact: %w", err)
			}
		}
		output := normalizeJSONStepOutput(parsed, finalRaw)
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		if err := writeParsed(stepDir, output); err != nil {
			return err
		}
		return nil
	}
	if err := os.WriteFile(filepath.Join(stepDir, "stdout.txt"), []byte(response.Raw), 0o644); err != nil {
		return fmt.Errorf("write stdout artifact: %w", err)
	}
	if step.OutputMode == AgentOutputText {
		output := map[string]any{"output_mode": string(AgentOutputText), "status": "captured", "stdout": response.Raw}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	return fmt.Errorf("unsupported agent output mode %q", step.OutputMode)
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
