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
}

// Runner executes typed flow definitions and writes per-step audit artifacts.
type Runner struct {
	Agent        ports.AgentAdapter
	ArtifactRoot string
	RunID        string
	Verifier     ports.Verifier
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
		stepResult, err := r.executeStep(ctx, step, &result)
		result.Steps = append(result.Steps, stepResult)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r Runner) executeStages(ctx context.Context, stages []Stage, result Result) (Result, error) {
	for _, stage := range stages {
		for _, step := range stage.Setup {
			stepResult, err := r.executeStep(ctx, step, &result)
			result.Steps = append(result.Steps, stepResult)
			if err != nil {
				return result, fmt.Errorf("stage %q setup: %w", stage.ID, err)
			}
		}
		for _, step := range stage.Steps {
			stepResult, err := r.executeStep(ctx, step, &result)
			result.Steps = append(result.Steps, stepResult)
			if err != nil {
				return result, fmt.Errorf("stage %q step: %w", stage.ID, err)
			}
		}
		for _, step := range stage.After {
			stepResult, err := r.executeStep(ctx, step, &result)
			result.Steps = append(result.Steps, stepResult)
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
		if err := writeParsed(stepDir, map[string]string{"status": "not_implemented"}); err != nil {
			return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, err
		}
	default:
		return StepResult{ID: step.ID, Type: step.Type, Status: StepFailed}, fmt.Errorf("unsupported step type %q", step.Type)
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

func writeInboxItem(root string, runID string, stepID string, value any) error {
	path := filepath.Join(root, ".dft", "inbox", runID+"-"+stepID+".json")
	return writeJSONArtifact(path, value)
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
