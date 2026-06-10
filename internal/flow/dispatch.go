package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bocacorazon/dft/internal/agentjson"
	"github.com/bocacorazon/dft/internal/ports"
)

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
	handler, ok := functionHandlers[step.Function]
	if !ok {
		return fmt.Errorf("unsupported function %q", step.Function)
	}
	return handler(r, ctx, step, root, stepDir, result)
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
		Increment:  step.Increment,
		RunID:      r.RunID,
		Cwd:        step.Cwd,
		Env:        step.Env,
		Model:      step.Model,
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
