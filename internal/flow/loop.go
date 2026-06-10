package flow

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
)

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
