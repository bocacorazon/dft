package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestRunBuildExecutesSingleSpecFlow(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(".dft", "flows"), 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(".dft", "flows", "single-spec.yaml"), []byte(flowContent), 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}
	models := domain.ModelConfig{
		DefaultFamily: "openai",
		Families: map[string]domain.ModelFamilyConfig{
			"openai": {Low: "gpt-5-mini"},
		},
	}
	content, err := json.Marshal(models)
	if err != nil {
		t.Fatalf("marshal models: %v", err)
	}
	if err := os.WriteFile("models.json", content, 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"build", "--prompt", "Build auth", "--workflow", "single-spec", "--feature-slug", "feature-auth", "--increment-branch", "increment/feature-auth", "--models", "models.json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned exit code %d\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "completed spec feature-auth") {
		t.Fatalf("stdout = %q, want build completion summary", stdout.String())
	}
	if _, err := os.Stat(filepath.Join("specs", "feature-auth", "spec.md")); err != nil {
		t.Fatalf("spec artifact missing: %v", err)
	}
	resultPaths, err := filepath.Glob(filepath.Join(".dft", "runs", "*", "execution-result.json"))
	if err != nil {
		t.Fatalf("glob execution result: %v", err)
	}
	if len(resultPaths) != 1 {
		t.Fatalf("execution result count = %d, want 1", len(resultPaths))
	}
	resultContent, err := os.ReadFile(resultPaths[0])
	if err != nil {
		t.Fatalf("read execution result: %v", err)
	}
	var result domain.ExecutionResult
	if err := json.Unmarshal(resultContent, &result); err != nil {
		t.Fatalf("unmarshal execution result: %v", err)
	}
	if result.IncrementBranch != "increment/feature-auth" {
		t.Fatalf("increment branch = %q, want increment/feature-auth", result.IncrementBranch)
	}
}

func TestRunSubmitExecutesWBSDAG(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(".dft", "flows"), 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(".dft", "flows", "single-spec.yaml"), []byte(flowContent), 0o644); err != nil {
		t.Fatalf("write flow: %v", err)
	}
	if err := os.WriteFile("models.json", []byte(`{"default_family":"openai","families":{"openai":{"low":"gpt-5-mini"}}}`), 0o644); err != nil {
		t.Fatalf("write models: %v", err)
	}
	wbs := domain.WBS{
		IncrementPackageID: "demo-run",
		BaseBranch:      "main",
		IncrementBranch: "increment/demo-run",
		Specs: []domain.SpecRef{
			{ID: "001-base", Description: "Base", AcceptanceCriteria: []string{"base"}},
			{ID: "002-auth", Description: "Auth", DependsOn: []string{"001-base"}, AcceptanceCriteria: []string{"auth"}},
		},
	}
	content, err := json.Marshal(wbs)
	if err != nil {
		t.Fatalf("marshal wbs: %v", err)
	}
	if err := os.WriteFile("wbs.json", content, 0o644); err != nil {
		t.Fatalf("write wbs: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--wbs", "wbs.json", "--workflow", "single-spec", "--models", "models.json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run returned exit code %d\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "2/2 specs complete") {
		t.Fatalf("stdout = %q, want orchestration summary", stdout.String())
	}
	if _, err := os.Stat(filepath.Join("specs", "001-base", "spec.md")); err != nil {
		t.Fatalf("base spec artifact missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join("specs", "002-auth", "spec.md")); err != nil {
		t.Fatalf("dependent spec artifact missing: %v", err)
	}
	resultPaths, err := filepath.Glob(filepath.Join(".dft", "runs", "*", "execution-orchestration.json"))
	if err != nil {
		t.Fatalf("glob orchestration result: %v", err)
	}
	if len(resultPaths) != 1 {
		t.Fatalf("orchestration result count = %d, want 1", len(resultPaths))
	}
	orchestrationContent, err := os.ReadFile(resultPaths[0])
	if err != nil {
		t.Fatalf("read orchestration result: %v", err)
	}
	var result domain.ExecutionOrchestrationResult
	if err := json.Unmarshal(orchestrationContent, &result); err != nil {
		t.Fatalf("unmarshal orchestration result: %v", err)
	}
	if result.BaseBranch != "main" || result.IncrementBranch != "increment/demo-run" {
		t.Fatalf("result branches = %#v, want main/increment/demo-run", result)
	}
}
