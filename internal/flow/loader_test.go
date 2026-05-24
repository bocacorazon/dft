package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinitionReadsExternalFlowFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flow.yaml")
	content := `schema_version: "1.0"
workflow:
  id: "review"
  name: "Review flow"
max_spec_parallelism: 2
steps:
  - id: review
    type: agent
    agent_name: dft-code-review.agent.md
    prompt: Review
    demand: x
    max_iterations: 2
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}

	definition, err := LoadDefinition(path)

	if err != nil {
		t.Fatalf("LoadDefinition returned error: %v", err)
	}
	if definition.MaxSpecParallelism != 2 {
		t.Fatalf("parallelism = %d, want 2", definition.MaxSpecParallelism)
	}
	if len(definition.Steps) != 1 || definition.Steps[0].MaxIterations != 2 {
		t.Fatalf("steps = %#v, want bounded loop step", definition.Steps)
	}
}

func TestLoadDefinitionMapsWorkflowYAMLCommandSyntax(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speckit.yaml")
	content := `schema_version: "1.0"
workflow:
  id: "speckit"
steps:
  - id: specify
    command: speckit.specify
    integration: copilot
    model: gpt-5-mini
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "{{ inputs.feature_directory }}"
    input:
      args: "{{ inputs.specify_input }}"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}

	definition, err := LoadDefinition(path)
	if err != nil {
		t.Fatalf("LoadDefinition returned error: %v", err)
	}
	if len(definition.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(definition.Steps))
	}
	step := definition.Steps[0]
	if step.Type != StepCommand || step.CommandName != "speckit.specify" {
		t.Fatalf("step = %#v, want command step", step)
	}
	if step.CommandInput != "{{ inputs.specify_input }}" {
		t.Fatalf("command input = %q", step.CommandInput)
	}
}
