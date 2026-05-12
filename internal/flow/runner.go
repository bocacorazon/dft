package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/adapters/github"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// StepType names the kind of work a typed flow step performs.
type StepType string

const (
	StepAgent    StepType = "agent"
	StepFunction StepType = "function"
	StepTool     StepType = "tool"
	StepVerify   StepType = "verify"
	StepWorkflow StepType = "workflow"
	StepLoop     StepType = "loop"
)

// StepStatus captures the terminal state of an executed step.
type StepStatus string

const (
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
)

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
	AgentName     string            `json:"agent_name,omitempty"`
	Prompt        string            `json:"prompt,omitempty"`
	Demand        string            `json:"demand,omitempty"`
	Cwd           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Function      string            `json:"function,omitempty"`
	Args          map[string]string `json:"args,omitempty"`
	MaxIterations int               `json:"max_iterations,omitempty"`
	NoContext     bool              `json:"no_context,omitempty"`
	Setup         []Step            `json:"setup,omitempty"`
	Verify        []domain.Check    `json:"verify,omitempty"`
	Checks        []domain.Check    `json:"checks,omitempty"`
	OnError       string            `json:"on_error,omitempty"`
	Workflow      string            `json:"workflow,omitempty"`
	Steps         []Step            `json:"steps,omitempty"`
	ExitWhen      map[string]string `json:"exit_when,omitempty"`
}

// Runner executes typed flow definitions and writes per-step audit artifacts.
type Runner struct {
	Agent            ports.AgentAdapter
	ArtifactRoot     string
	RunID            string
	Verifier         ports.Verifier
	CommitLocalSteps bool
}

// Result contains terminal status for every completed step.
type Result struct {
	Steps        []StepResult
	Vars         map[string]string
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
		Steps: make([]StepResult, 0, len(definition.Steps)),
		Vars:  map[string]string{},
	}
	if len(definition.Stages) > 0 {
		return r.executeStages(ctx, definition.Stages, result)
	}
	for _, step := range definition.Steps {
		stepResults, err := r.executeStepWithPolicy(ctx, step, &result)
		result.Steps = append(result.Steps, stepResults...)
		if err != nil {
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
		stepResult, err := r.executeStep(ctx, rendered, result)
		stepResults = append(stepResults, stepResult)
		if err == nil {
			return stepResults, nil
		}
		lastErr = err
	}

	switch onErrorMode(step.OnError) {
	case "continue":
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

	switch step.Type {
	case StepAgent:
		if err := r.executeAgentStep(ctx, step, stepDir); err != nil {
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
	if r.CommitLocalSteps && mutatesLocal(step) {
		if _, err := commitStep(ctx, commitRoot(r.ArtifactRoot, step), r.RunID, step.ID); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	}

	return StepResult{ID: step.ID, Type: step.Type, Status: StepSucceeded}, nil
}

func (r Runner) executeFunctionStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	switch step.Function {
	case "set_var":
		name := step.Args["name"]
		value := step.Args["value"]
		if name == "" {
			return fmt.Errorf("set_var requires name")
		}
		result.Vars[name] = value
		return writeParsed(stepDir, map[string]string{name: value})
	case "gh_pr_create":
		record, err := githubAdapter(r.ArtifactRoot, step).CreatePR(ctx, github.PRRequest{
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
		return writeParsed(stepDir, record)
	case "gh_pr_number_for_branch":
		record, err := githubAdapter(r.ArtifactRoot, step).PRNumberForBranch(ctx, github.BranchPRRequest{
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
		return writeParsed(stepDir, record)
	case "gh_pr_wait_checks":
		number, err := stepPRNumber(step, result)
		if err != nil {
			return err
		}
		record, err := githubAdapter(r.ArtifactRoot, step).WaitChecks(ctx, github.CheckRequest{RunID: r.RunID, StepID: step.ID, Number: number})
		if err != nil {
			return err
		}
		return writeParsed(stepDir, record)
	case "gh_pr_merge":
		number, err := stepPRNumber(step, result)
		if err != nil {
			return err
		}
		record, err := githubAdapter(r.ArtifactRoot, step).MergePR(ctx, github.MergeRequest{RunID: r.RunID, StepID: step.ID, Number: number, Method: step.Args["method"]})
		if err != nil {
			return err
		}
		return writeParsed(stepDir, record)
	case "wait_for_human":
		record := map[string]string{
			"status":  "blocked",
			"message": step.Args["message"],
		}
		if err := writeInboxItem(r.ArtifactRoot, r.RunID, step.ID, record); err != nil {
			return err
		}
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
		return writeParsed(stepDir, map[string]string{"commit_message": message})
	case "git_branch_current":
		out, err := runGit(ctx, r.ArtifactRoot, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return err
		}
		branch := strings.TrimSpace(out)
		result.Vars["current_branch"] = branch
		return writeParsed(stepDir, map[string]string{"current_branch": branch})
	case "git_default_branch":
		branch, err := defaultBranch(ctx, r.ArtifactRoot)
		if err != nil {
			return err
		}
		result.Vars["default_branch"] = branch
		return writeParsed(stepDir, map[string]string{"default_branch": branch})
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
		if _, err := runGit(ctx, r.ArtifactRoot, args...); err != nil {
			return err
		}
		return writeParsed(stepDir, map[string]string{"branch": name, "base": base})
	case "merge":
		source := step.Args["source"]
		target := step.Args["target"]
		if source == "" || target == "" {
			return fmt.Errorf("merge requires source and target")
		}
		if _, err := runGit(ctx, r.ArtifactRoot, "switch", target); err != nil {
			return err
		}
		if _, err := runGit(ctx, r.ArtifactRoot, "merge", "--no-ff", "--no-edit", source); err != nil {
			return err
		}
		return writeParsed(stepDir, map[string]string{"source": source, "target": target})
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
			if err := writeRemoteAudit(r.ArtifactRoot, r.RunID, step.ID, record); err != nil {
				return err
			}
			return writeParsed(stepDir, record)
		}
		if _, err := runGit(ctx, r.ArtifactRoot, "push", remote, branch); err != nil {
			return err
		}
		record["status"] = "pushed"
		if err := writeRemoteAudit(r.ArtifactRoot, r.RunID, step.ID, record); err != nil {
			return err
		}
		return writeParsed(stepDir, record)
	default:
		return fmt.Errorf("unsupported function %q", step.Function)
	}
}

func (r Runner) executeVerifyStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	checks := step.Checks
	if len(checks) == 0 {
		checks = step.Verify
	}
	if len(checks) == 0 {
		return fmt.Errorf("verify step %q requires checks", step.ID)
	}
	if r.Verifier == nil {
		return fmt.Errorf("verify step %q verifier is required", step.ID)
	}
	verification := r.Verifier.Run(ctx, checks)
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
	if r.Verifier == nil {
		return fmt.Errorf("step %q verifier is required", step.ID)
	}
	verification := r.Verifier.Run(ctx, step.Verify)
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
			return writeParsed(stepDir, map[string]any{"status": "succeeded", "iterations": i + 1})
		}
	}
	if len(step.ExitWhen) > 0 {
		return fmt.Errorf("loop step %q exhausted %d iteration(s)", step.ID, step.MaxIterations)
	}
	return writeParsed(stepDir, map[string]any{"status": "succeeded", "iterations": step.MaxIterations})
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
	step.Prompt = renderString(step.Prompt, result)
	step.Demand = renderString(step.Demand, result)
	step.Cwd = renderString(step.Cwd, result)
	step.Workflow = renderString(step.Workflow, result)
	for i, value := range step.Command {
		step.Command[i] = renderString(value, result)
	}
	for key, value := range step.Env {
		step.Env[key] = renderString(value, result)
	}
	for key, value := range step.Args {
		step.Args[key] = renderString(value, result)
	}
	for key, value := range step.ExitWhen {
		step.ExitWhen[key] = renderString(value, result)
	}
	return step
}

func renderString(value string, result *Result) string {
	if value == "" || result == nil {
		return value
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

func mutatesLocal(step Step) bool {
	if step.Type == StepTool || step.Type == StepAgent {
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
	if step.Cwd != "" && (step.Type == StepAgent || step.Type == StepTool) {
		return step.Cwd
	}
	return root
}

func commitStep(ctx context.Context, root string, runID string, stepID string) (string, error) {
	status, err := runGit(ctx, root, "status", "--porcelain")
	if err != nil {
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

func (r Runner) executeAgentStep(ctx context.Context, step Step, stepDir string) error {
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
	response, err := r.Agent.Invoke(ctx, ports.AgentRequest{
		AgentName: step.AgentName,
		Prompt:    prompt,
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
