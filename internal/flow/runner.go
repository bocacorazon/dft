package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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
	ModelType     string            `json:"model_type,omitempty"`
	AgentName     string            `json:"agent_name,omitempty"`
	OutputMode    AgentOutputMode   `json:"output_mode,omitempty"`
	AllowTools    bool              `json:"allow_tools,omitempty"`
	Prompt        string            `json:"prompt,omitempty"`
	Increment     string            `json:"increment,omitempty"`
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
	step.ModelType = renderString(step.ModelType, result)
	step.Prompt = renderString(step.Prompt, result)
	step.Increment = renderString(step.Increment, result)
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
