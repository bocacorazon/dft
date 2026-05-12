package flow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/ports"
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

func TestRunnerAttachesProjectContextAndWritesContextHashes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dft", "context"), 0o755); err != nil {
		t.Fatalf("create context dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dft", "context", "project.md"), []byte("Use mandatory TDD.\n"), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	agent := &capturingAgent{}
	runner := Runner{
		Agent:        agent,
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	_, err := runner.Execute(context.Background(), Definition{
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
	if !strings.Contains(agent.prompt, "Use mandatory TDD.") {
		t.Fatalf("agent prompt did not include project context: %q", agent.prompt)
	}
	stepDir := filepath.Join(root, ".dft", "runs", "run-123", "steps", "intake")
	content, err := os.ReadFile(filepath.Join(stepDir, "context-hashes.json"))
	if err != nil {
		t.Fatalf("read context hashes: %v", err)
	}
	var artifact struct {
		Context []contextHash `json:"context"`
	}
	if err := json.Unmarshal(content, &artifact); err != nil {
		t.Fatalf("context hashes invalid JSON: %v\n%s", err, content)
	}
	if len(artifact.Context) != 1 || artifact.Context[0].Path != ".dft/context/project.md" || len(artifact.Context[0].SHA256) != 64 {
		t.Fatalf("context hashes = %#v, want project context hash", artifact.Context)
	}
}

func TestRunnerCanDisableProjectContext(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dft", "context"), 0o755); err != nil {
		t.Fatalf("create context dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dft", "context", "project.md"), []byte("Use mandatory TDD.\n"), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	agent := &capturingAgent{}
	runner := Runner{Agent: agent, ArtifactRoot: root, RunID: "run-123"}

	_, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:        "intake",
			Type:      StepAgent,
			AgentName: "dft-intake.agent.md",
			Prompt:    "Normalize demand",
			NoContext: true,
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(agent.prompt, "Use mandatory TDD.") {
		t.Fatalf("agent prompt included context despite no_context: %q", agent.prompt)
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

type capturingAgent struct {
	prompt string
}

func (a *capturingAgent) Invoke(_ context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	a.prompt = request.Prompt
	return ports.AgentResponse{Raw: `{"ok":true}`}, nil
}
