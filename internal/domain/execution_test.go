package domain

import (
	"strings"
	"testing"
)

func TestExecutionRequestValidateRequiresSinglePromptSource(t *testing.T) {
	req := ExecutionRequest{
		Prompt:          "Build auth",
		PromptPath:      "specs/auth.md",
		Workflow:        "spec-exec",
		FeatureSlug:     "auth",
		ModelConfigPath: "models.json",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("Validate returned nil error, want prompt source validation")
	}
}

func TestExecutionRequestValidateAcceptsPromptFile(t *testing.T) {
	req := ExecutionRequest{
		PromptPath:      "specs/auth.md",
		WorkflowPath:    ".dft/flows/spec.yaml",
		FeatureSlug:     "auth",
		ModelConfigPath: "models.json",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestExecutionRequestValidateRejectsInvalidIncrementBranch(t *testing.T) {
	req := ExecutionRequest{
		Prompt:          "Build auth",
		Workflow:        "spec-exec",
		FeatureSlug:     "auth",
		IncrementBranch: "bad branch",
		ModelConfigPath: "models.json",
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("Validate returned nil error, want invalid increment branch failure")
	}
	if !strings.Contains(err.Error(), "increment branch") {
		t.Fatalf("error = %q, want increment branch validation", err)
	}
}

func TestOrchestrationRequestValidateRejectsInvalidBaseBranch(t *testing.T) {
	req := OrchestrationRequest{
		WBSPath:         "wbs.json",
		BaseBranch:      "bad branch",
		ModelConfigPath: "models.json",
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("Validate returned nil error, want invalid base branch failure")
	}
	if !strings.Contains(err.Error(), "base branch") {
		t.Fatalf("error = %q, want base branch validation", err)
	}
}

func TestModelConfigResolveUsesDefaultFamily(t *testing.T) {
	cfg := ModelConfig{
		DefaultFamily: "openai",
		Families: map[string]ModelFamilyConfig{
			"openai": {Low: "gpt-5-mini"},
		},
	}
	model, err := cfg.Resolve("", ModelTierLow)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if model != "gpt-5-mini" {
		t.Fatalf("model = %q, want gpt-5-mini", model)
	}
}

func TestModelConfigResolveRequiresFamilyWhenAmbiguous(t *testing.T) {
	cfg := ModelConfig{
		Families: map[string]ModelFamilyConfig{
			"openai":    {Low: "gpt-5-mini"},
			"anthropic": {Low: "claude-haiku-4.5"},
		},
	}
	if _, err := cfg.Resolve("", ModelTierLow); err == nil {
		t.Fatal("Resolve returned nil error, want family disambiguation failure")
	}
}
