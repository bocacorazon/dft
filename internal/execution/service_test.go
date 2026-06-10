package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestServiceExecuteRunsNamedWorkflow(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dft", "flows"), 0o755); err != nil {
		t.Fatalf("mkdir flows: %v", err)
	}
	flowContent := `schema_version: "1.0"
steps:
  - id: specify
    command: speckit.specify
    integration: stub
    model_type: low
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "specs/{{ inputs.feature_slug }}"
    input:
      args: "{{ inputs.prompt }}"
`
	if err := os.WriteFile(filepath.Join(root, ".dft", "flows", "single-spec.yaml"), []byte(flowContent), 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}
	models := domain.ModelConfig{
		DefaultFamily: "openai",
		Families: map[string]domain.ModelFamilyConfig{
			"openai": {Low: "gpt-5-mini"},
		},
	}
	modelContent, err := json.Marshal(models)
	if err != nil {
		t.Fatalf("marshal model config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "models.json"), modelContent, 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}
	service := Service{
		Agent:        agentstub.Adapter{},
		Dispatcher:   agentstub.Adapter{},
		Verifier:     verify.Checker{RootDir: root},
		ArtifactRoot: root,
	}
	result, err := service.Execute(context.Background(), "run-123", "001-auth", domain.ExecutionRequest{
		Prompt:          "Build auth",
		Workflow:        "single-spec",
		FeatureSlug:     "feature-auth",
		IncrementBranch: "increment/feature-auth",
		ModelConfigPath: "models.json",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.BaseBranch != "increment/feature-auth" {
		t.Fatalf("base branch = %q, want increment/feature-auth", result.BaseBranch)
	}
	if result.IncrementBranch != "increment/feature-auth" {
		t.Fatalf("increment branch = %q, want increment/feature-auth", result.IncrementBranch)
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "feature-auth", "spec.md")); err != nil {
		t.Fatalf("spec artifact missing: %v", err)
	}
}

func TestServiceExecuteDefaultsIncrementBranchFromGit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dft", "flows"), 0o755); err != nil {
		t.Fatalf("mkdir flows: %v", err)
	}
	flowContent := `schema_version: "1.0"
steps:
  - id: specify
    command: speckit.specify
    integration: stub
    model_type: low
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "specs/{{ inputs.feature_slug }}"
    input:
      args: "{{ inputs.prompt }}"
`
	if err := os.WriteFile(filepath.Join(root, ".dft", "flows", "single-spec.yaml"), []byte(flowContent), 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "models.json"), []byte(`{"default_family":"openai","families":{"openai":{"low":"gpt-5-mini"}}}`), 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}
	service := Service{
		Agent:        agentstub.Adapter{},
		Dispatcher:   agentstub.Adapter{},
		Verifier:     verify.Checker{RootDir: root},
		Git:          fakeGit{defaultBranch: "main"},
		ArtifactRoot: root,
	}
	result, err := service.Execute(context.Background(), "run-456", "001-auth", domain.ExecutionRequest{
		Prompt:          "Build auth",
		Workflow:        "single-spec",
		FeatureSlug:     "feature-auth",
		ModelConfigPath: "models.json",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.BaseBranch != "main" {
		t.Fatalf("base branch = %q, want main", result.BaseBranch)
	}
	if result.IncrementBranch != "main" {
		t.Fatalf("increment branch = %q, want main", result.IncrementBranch)
	}
}

func TestOrchestratorExecuteHonorsDependenciesAndPostsCallbacks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dft", "flows"), 0o755); err != nil {
		t.Fatalf("mkdir flows: %v", err)
	}
	flowContent := `schema_version: "1.0"
steps:
  - id: specify
    command: speckit.specify
    integration: stub
    model_type: low
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "specs/{{ inputs.feature_slug }}"
    input:
      args: "{{ inputs.prompt }}"
`
	if err := os.WriteFile(filepath.Join(root, ".dft", "flows", "single-spec.yaml"), []byte(flowContent), 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}
	models := `{"default_family":"openai","families":{"openai":{"low":"gpt-5-mini"}}}`
	if err := os.WriteFile(filepath.Join(root, "models.json"), []byte(models), 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}
	wbs := domain.WBS{
		IncrementPackageID: "run-123",
		BaseBranch:         "main",
		IncrementBranch:    "increment/run-123",
		Specs: []domain.SpecRef{
			{ID: "001-base", Description: "Base", AcceptanceCriteria: []string{"base"}},
			{ID: "002-auth", Description: "Auth", DependsOn: []string{"001-base"}, AcceptanceCriteria: []string{"auth"}},
		},
	}
	content, err := json.Marshal(wbs)
	if err != nil {
		t.Fatalf("marshal wbs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "wbs.json"), content, 0o644); err != nil {
		t.Fatalf("write wbs: %v", err)
	}
	var callbacks []domain.CallbackPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload domain.CallbackPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode callback: %v", err)
		}
		callbacks = append(callbacks, payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	orchestrator := Orchestrator{
		Registry: NewRegistry(Service{
			Agent:        agentstub.Adapter{},
			Dispatcher:   agentstub.Adapter{},
			Verifier:     verify.Checker{RootDir: root},
			ArtifactRoot: root,
		}),
		ArtifactRoot: root,
	}
	result, err := orchestrator.Execute(context.Background(), "run-123", domain.OrchestrationRequest{
		WBSPath:         "wbs.json",
		Workflow:        "single-spec",
		ModelConfigPath: "models.json",
		CompletionURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.BaseBranch != "main" {
		t.Fatalf("base branch = %q, want main", result.BaseBranch)
	}
	if result.IncrementBranch != "increment/run-123" {
		t.Fatalf("increment branch = %q, want increment/run-123", result.IncrementBranch)
	}
	if len(callbacks) != 3 {
		t.Fatalf("callback count = %d, want 3", len(callbacks))
	}
	if callbacks[0].SpecID != "001-base" || callbacks[1].SpecID != "002-auth" {
		t.Fatalf("callback order = %#v, want dependency order", callbacks)
	}
	if callbacks[0].IncrementBranch != "increment/run-123" || callbacks[2].BaseBranch != "main" {
		t.Fatalf("callbacks = %#v, want branch metadata", callbacks)
	}
}

type fakeGit struct {
	defaultBranch string
}

func (g fakeGit) DefaultBranch(context.Context) (string, error) {
	return g.defaultBranch, nil
}

func (fakeGit) CreateBranch(context.Context, ports.CreateBranchRequest) error {
	return nil
}

func (fakeGit) CreateWorktree(context.Context, ports.CreateWorktreeRequest) error {
	return nil
}

func (fakeGit) Merge(context.Context, ports.MergeRequest) error {
	return nil
}
