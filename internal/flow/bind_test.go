package flow

import "testing"

func TestBindInputsRendersWorkflowDataFromInputs(t *testing.T) {
	definition := Definition{
		Steps: []Step{
			{
				ID:           "specify",
				Type:         StepCommand,
				CommandName:  "speckit.specify",
				CommandInput: "{{ inputs.specify_input }}",
				Env: map[string]string{
					"SPECIFY_FEATURE_DIRECTORY": "{{ inputs.feature_directory }}",
				},
			},
			{
				ID:      "review-spec",
				Type:    StepGate,
				Message: "{{ inputs.review_message }}",
			},
		},
	}

	bound := BindInputs(definition, map[string]any{
		"specify_input":     "Enable real submit",
		"feature_directory": "specs/001-real-submit",
		"review_message":    "Review the generated spec before planning.",
	})

	if got := bound.Steps[0].CommandInput; got != "Enable real submit" {
		t.Fatalf("command input = %q", got)
	}
	if got := bound.Steps[0].Env["SPECIFY_FEATURE_DIRECTORY"]; got != "specs/001-real-submit" {
		t.Fatalf("feature directory = %q", got)
	}
	if got := bound.Steps[1].Message; got != "Review the generated spec before planning." {
		t.Fatalf("gate message = %q", got)
	}
}

func TestBindDefinitionAppliesExecutionContextWithoutChangingGates(t *testing.T) {
	definition := Definition{
		Steps: []Step{
			{
				ID:          "specify",
				Type:        StepCommand,
				CommandName: "speckit.specify",
				Env: map[string]string{
					"SPECIFY_FEATURE_DIRECTORY": "specs/001-auth",
				},
			},
			{
				ID:      "review-spec",
				Type:    StepGate,
				Message: "Review the generated spec before planning.",
			},
		},
	}

	bound := BindDefinition(definition, ExecutionContext{
		Cwd: ".dft/worktrees/run-123/001-auth",
		Env: map[string]string{
			"GIT_BRANCH_NAME": "spec/run-123/001-auth",
		},
	})

	if got := bound.Steps[0].Cwd; got != ".dft/worktrees/run-123/001-auth" {
		t.Fatalf("command Cwd = %q", got)
	}
	if got := bound.Steps[0].Env["SPECIFY_FEATURE_DIRECTORY"]; got != "specs/001-auth" {
		t.Fatalf("command SPECIFY_FEATURE_DIRECTORY = %q", got)
	}
	if got := bound.Steps[0].Env["GIT_BRANCH_NAME"]; got != "spec/run-123/001-auth" {
		t.Fatalf("command GIT_BRANCH_NAME = %q", got)
	}
	if got := bound.Steps[1].Cwd; got != "" {
		t.Fatalf("gate Cwd = %q, want empty", got)
	}
	if bound.Steps[1].Env != nil {
		t.Fatalf("gate Env = %#v, want nil", bound.Steps[1].Env)
	}
}
