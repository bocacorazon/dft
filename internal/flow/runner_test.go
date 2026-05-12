package flow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
)

func TestRunnerExecutesAgentStepAndWritesAuditArtifacts(t *testing.T) {
	root := t.TempDir()
	runner := Runner{
		Agent:        agentstub.Adapter{},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	result, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:        "intake",
			Type:      StepAgent,
			AgentName: "dft-intake.agent.md",
			Prompt:    "Normalize demand",
			Demand:    "Build intake loop",
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("step result count = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].Status != StepSucceeded {
		t.Fatalf("step status = %q, want succeeded", result.Steps[0].Status)
	}

	stepDir := filepath.Join(root, ".dft", "runs", "run-123", "steps", "intake")
	for _, name := range []string{"prompt.md", "stdout.txt", "parsed.json"} {
		path := filepath.Join(stepDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", name, err)
		}
		assertJSONWhenParsedArtifact(t, path)
	}
}

func TestRunnerStopsOnFailedStep(t *testing.T) {
	runner := Runner{RunID: "run-123", ArtifactRoot: t.TempDir()}

	_, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:   "broken",
			Type: StepAgent,
		}},
	})

	if err == nil {
		t.Fatal("Execute returned nil error, want failure")
	}
}

func assertJSONWhenParsedArtifact(t *testing.T, path string) {
	t.Helper()
	if filepath.Base(path) != "parsed.json" {
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read parsed artifact: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("parsed artifact is invalid JSON: %v\n%s", err, content)
	}
}
