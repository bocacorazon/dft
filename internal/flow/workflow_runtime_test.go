package flow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/ports"
)

func TestRunnerExecutesCommandStepAndCapturesStepOutput(t *testing.T) {
	root := t.TempDir()
	dispatcher := &capturingDispatcher{
		response: map[string]portsCommandResponse{
			"test.command": {Stdout: "spec generated\n", ExitCode: 0},
		},
	}
	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
		Dispatcher:   dispatcher,
		Inputs: map[string]any{
			"spec": "Build auth",
		},
	}

	result, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:           "specify",
			Type:         StepCommand,
			CommandName:  "test.command",
			CommandInput: "{{ inputs.spec }}",
			Integration:  "copilot",
			Model:        "gpt-5-mini",
		}},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if dispatcher.requests[0].Input != "Build auth" {
		t.Fatalf("command input = %q, want rendered input", dispatcher.requests[0].Input)
	}
	if got := result.StepOutputs["specify"]["stdout"]; got != "spec generated\n" {
		t.Fatalf("step output stdout = %#v, want captured stdout", got)
	}

	var state runState
	content, err := os.ReadFile(filepath.Join(root, ".dft", "runs", "run-123", "state.json"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if err := json.Unmarshal(content, &state); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if state.Status != "completed" || state.CurrentStepIndex != 1 {
		t.Fatalf("state = %#v, want completed at step 1", state)
	}
}

func TestRunnerCommandStepFailsWhenSpecKitArtifactsAreMissing(t *testing.T) {
	root := t.TempDir()
	dispatcher := &capturingDispatcher{
		response: map[string]portsCommandResponse{
			"speckit.specify": {Stdout: "done\n", ExitCode: 0},
		},
	}
	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
		Dispatcher:   dispatcher,
	}

	_, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:           "specify",
			Type:         StepCommand,
			CommandName:  "speckit.specify",
			CommandInput: "Build auth",
			Cwd:          root,
			Env: map[string]string{
				"SPECIFY_FEATURE_DIRECTORY": "specs/001-auth",
			},
			NoContext: true,
		}},
	})
	if err == nil {
		t.Fatal("Execute returned nil error, want missing artifact failure")
	}
	if got := err.Error(); !strings.Contains(got, "spec.md") {
		t.Fatalf("error = %q, want missing spec.md context", got)
	}
}

func TestRunnerCommandStepCapturesSpecKitArtifacts(t *testing.T) {
	root := t.TempDir()
	dispatcher := &capturingDispatcher{
		response: map[string]portsCommandResponse{
			"speckit.specify": {Stdout: "done\n", ExitCode: 0},
		},
		onDispatch: func(request ports.CommandRequest) error {
			featureDir := filepath.Join(request.Cwd, request.Env["SPECIFY_FEATURE_DIRECTORY"])
			if err := os.MkdirAll(filepath.Join(featureDir, "checklists"), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(featureDir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(featureDir, "checklists", "requirements.md"), []byte("# Checklist\n"), 0o644)
		},
	}
	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
		Dispatcher:   dispatcher,
	}

	result, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:           "specify",
			Type:         StepCommand,
			CommandName:  "speckit.specify",
			CommandInput: "Build auth",
			Cwd:          root,
			Env: map[string]string{
				"SPECIFY_FEATURE_DIRECTORY": "specs/001-auth",
			},
			NoContext: true,
		}},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	artifacts, ok := result.StepOutputs["specify"]["artifacts"].(map[string]any)
	if !ok {
		t.Fatalf("artifacts = %#v, want captured artifact map", result.StepOutputs["specify"]["artifacts"])
	}
	if got := artifacts["spec_file"]; got != filepath.ToSlash(filepath.Join(root, "specs", "001-auth", "spec.md")) {
		t.Fatalf("spec_file = %#v", got)
	}
}

func TestRunnerPausesAtGateAndResumeContinuesExecution(t *testing.T) {
	root := t.TempDir()
	runner := Runner{ArtifactRoot: root, RunID: "run-123"}
	definition := Definition{
		Steps: []Step{
			{ID: "review-spec", Type: StepGate, Message: "Review the generated spec before planning."},
			{ID: "after", Type: StepFunction, Function: "set_var", Args: map[string]string{"name": "status", "value": "continued"}},
		},
	}

	_, err := runner.Execute(context.Background(), definition)
	if err == nil {
		t.Fatal("Execute returned nil error, want pause")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".dft", "inbox", "run-123-review-spec.json")); statErr != nil {
		t.Fatalf("gate inbox item missing: %v", statErr)
	}

	resumed, err := (Runner{
		ArtifactRoot:     root,
		RunID:            "run-123",
		AutoApproveGates: true,
	}).Resume(context.Background(), definition)
	if err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
	if resumed.Vars["status"] != "continued" {
		t.Fatalf("vars = %#v, want resumed function output", resumed.Vars)
	}
}

func TestRunnerResumeFromOverridesSavedStepIndex(t *testing.T) {
	root := t.TempDir()
	runner := Runner{ArtifactRoot: root, RunID: "run-123"}
	definition := Definition{
		Steps: []Step{
			{ID: "first", Type: StepFunction, Function: "set_var", Args: map[string]string{"name": "first", "value": "done"}},
			{ID: "second", Type: StepFunction, Function: "set_var", Args: map[string]string{"name": "second", "value": "done"}},
			{ID: "third", Type: StepFunction, Function: "set_var", Args: map[string]string{"name": "third", "value": "done"}},
		},
	}
	if err := runner.saveState(runState{
		Status:           "failed",
		CurrentStepIndex: 3,
		Inputs:           map[string]any{},
		Vars:             map[string]string{"first": "done"},
		StepOutputs:      map[string]map[string]any{},
	}); err != nil {
		t.Fatalf("saveState returned error: %v", err)
	}

	resumed, err := runner.ResumeFrom(context.Background(), definition, 1)
	if err != nil {
		t.Fatalf("ResumeFrom returned error: %v", err)
	}
	if resumed.Vars["first"] != "done" || resumed.Vars["second"] != "done" || resumed.Vars["third"] != "done" {
		t.Fatalf("vars = %#v, want saved state plus resumed steps", resumed.Vars)
	}
}

func TestRenderStringUsesPriorStepOutputs(t *testing.T) {
	root := t.TempDir()
	dispatcher := &capturingDispatcher{
		response: map[string]portsCommandResponse{
			"test.plan": {Stdout: "plan complete", ExitCode: 0},
		},
	}
	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
		Dispatcher:   dispatcher,
	}

	result, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{
			{ID: "plan", Type: StepCommand, CommandName: "test.plan", CommandInput: "Build plan"},
			{ID: "capture", Type: StepFunction, Function: "set_var", Args: map[string]string{"name": "plan_output", "value": "{{ steps.plan.output.stdout }}"}},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Vars["plan_output"] != "plan complete" {
		t.Fatalf("plan_output = %q, want rendered step output", result.Vars["plan_output"])
	}
}

func TestRunnerNotifiesObserverOfStepLifecycle(t *testing.T) {
	root := t.TempDir()
	observer := &recordingObserver{}
	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
		Observer:     observer,
	}

	_, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:       "capture",
			Type:     StepFunction,
			Function: "set_var",
			Args: map[string]string{
				"name":  "status",
				"value": "ok",
			},
		}},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(observer.started) != 1 || observer.started[0] != "capture" {
		t.Fatalf("started = %#v, want capture", observer.started)
	}
	if len(observer.completed) != 1 || observer.completed[0] != "capture:succeeded" {
		t.Fatalf("completed = %#v, want capture:succeeded", observer.completed)
	}
}

func TestVerifySpeckitCommandArtifactsForPlanTasksAndImplement(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "specs", "001-auth")
	write := func(path string, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(filepath.Join(featureDir, "plan.md"), "# Plan\n")
	write(filepath.Join(featureDir, "research.md"), "# Research\n")
	write(filepath.Join(featureDir, "tasks.md"), "- [X] Complete task\n")

	cases := []struct {
		name    string
		command string
	}{
		{name: "plan", command: "speckit.plan"},
		{name: "tasks", command: "speckit.tasks"},
		{name: "implement", command: "speckit.implement"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			artifacts, err := verifySpeckitCommandArtifacts(root, Step{
				CommandName: tc.command,
				Cwd:         root,
				Env: map[string]string{
					"SPECIFY_FEATURE_DIRECTORY": "specs/001-auth",
				},
			})
			if err != nil {
				t.Fatalf("verifySpeckitCommandArtifacts returned error: %v", err)
			}
			if artifacts["feature_directory"] != filepath.ToSlash(featureDir) {
				t.Fatalf("feature_directory = %#v", artifacts["feature_directory"])
			}
			if tc.command == "speckit.plan" {
				if got := artifacts["contracts_dir"]; got != filepath.ToSlash(filepath.Join(featureDir, "contracts")) {
					t.Fatalf("contracts_dir = %#v", got)
				}
			}
		})
	}
}

func TestVerifySpeckitImplementArtifactsAcceptsLowercaseTaskMarkers(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "specs", "001-auth")
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("mkdir feature dir: %v", err)
	}
	tasksFile := filepath.Join(featureDir, "tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [x] Complete task\n"), 0o644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}

	artifacts, err := verifySpeckitCommandArtifacts(root, Step{
		CommandName: "speckit.implement",
		Cwd:         root,
		Env: map[string]string{
			"SPECIFY_FEATURE_DIRECTORY": "specs/001-auth",
		},
	})
	if err != nil {
		t.Fatalf("verifySpeckitCommandArtifacts returned error: %v", err)
	}
	if artifacts["completed_tasks"] != true {
		t.Fatalf("completed_tasks = %#v, want true", artifacts["completed_tasks"])
	}
}

type portsCommandResponse struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type capturingDispatcher struct {
	requests []struct {
		Command string
		Input   string
	}
	response   map[string]portsCommandResponse
	onDispatch func(ports.CommandRequest) error
}

type recordingObserver struct {
	started   []string
	completed []string
}

func (o *recordingObserver) StepStarted(_ string, step Step, _ Result) {
	o.started = append(o.started, step.ID)
}

func (o *recordingObserver) StepCompleted(_ string, step Step, status StepStatus, _ Result) {
	o.completed = append(o.completed, step.ID+":"+string(status))
}

func (d *capturingDispatcher) DispatchCommand(_ context.Context, request ports.CommandRequest) (ports.CommandResponse, error) {
	d.requests = append(d.requests, struct {
		Command string
		Input   string
	}{Command: request.Command, Input: request.Input})
	if d.onDispatch != nil {
		if err := d.onDispatch(request); err != nil {
			return ports.CommandResponse{}, err
		}
	}
	response := d.response[request.Command]
	return ports.CommandResponse{
		Stdout:   response.Stdout,
		Stderr:   response.Stderr,
		ExitCode: response.ExitCode,
	}, nil
}
