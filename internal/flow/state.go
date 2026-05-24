package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type runState struct {
	Status           string                    `json:"status"`
	CurrentStepIndex int                       `json:"current_step_index"`
	Inputs           map[string]any            `json:"inputs,omitempty"`
	Vars             map[string]string         `json:"vars,omitempty"`
	StepOutputs      map[string]map[string]any `json:"step_outputs,omitempty"`
}

func (r Runner) saveState(state runState) error {
	path := filepath.Join(r.ArtifactRoot, ".dft", "runs", r.RunID, "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run state: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write run state: %w", err)
	}
	return nil
}

func (r Runner) loadState() (runState, error) {
	path := filepath.Join(r.ArtifactRoot, ".dft", "runs", r.RunID, "state.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return runState{}, fmt.Errorf("read run state: %w", err)
	}
	var state runState
	if err := json.Unmarshal(content, &state); err != nil {
		return runState{}, fmt.Errorf("parse run state: %w", err)
	}
	if state.Inputs == nil {
		state.Inputs = map[string]any{}
	}
	if state.Vars == nil {
		state.Vars = map[string]string{}
	}
	if state.StepOutputs == nil {
		state.StepOutputs = map[string]map[string]any{}
	}
	return state, nil
}

func resultFromState(state runState) Result {
	return Result{
		Vars:        state.Vars,
		Inputs:      state.Inputs,
		StepOutputs: state.StepOutputs,
	}
}

func stateFromResult(result Result, index int, status string) runState {
	return runState{
		Status:           status,
		CurrentStepIndex: index,
		Inputs:           cloneAnyMap(result.Inputs),
		Vars:             cloneStringMap(result.Vars),
		StepOutputs:      cloneStepOutputs(result.StepOutputs),
	}
}

func cloneAnyMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	clone := make(map[string]any, len(value))
	for key, item := range value {
		clone[key] = item
	}
	return clone
}

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return map[string]string{}
	}
	clone := make(map[string]string, len(value))
	for key, item := range value {
		clone[key] = item
	}
	return clone
}

func cloneStepOutputs(value map[string]map[string]any) map[string]map[string]any {
	if len(value) == 0 {
		return map[string]map[string]any{}
	}
	clone := make(map[string]map[string]any, len(value))
	for key, output := range value {
		clone[key] = cloneAnyMap(output)
	}
	return clone
}
