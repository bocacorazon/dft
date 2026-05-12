package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinitionReadsExternalFlowFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flow.json")
	if err := os.WriteFile(path, []byte(`{"max_spec_parallelism":2,"steps":[{"id":"review","type":"agent","agent_name":"dft-code-review.agent.md","prompt":"Review","demand":"x","max_iterations":2}]}`), 0o644); err != nil {
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
