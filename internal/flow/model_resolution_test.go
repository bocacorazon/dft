package flow

import (
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestResolveModelsReplacesModelType(t *testing.T) {
	definition := Definition{
		Steps: []Step{{
			ID:          "specify",
			Type:        StepCommand,
			CommandName: "speckit.specify",
			ModelType:   "low",
		}},
	}
	cfg := domain.ModelConfig{
		DefaultFamily: "openai",
		Families: map[string]domain.ModelFamilyConfig{
			"openai": {Low: "gpt-5-mini"},
		},
	}
	resolved, err := ResolveModels(definition, cfg, "")
	if err != nil {
		t.Fatalf("ResolveModels returned error: %v", err)
	}
	if got := resolved.Steps[0].Model; got != "gpt-5-mini" {
		t.Fatalf("model = %q, want gpt-5-mini", got)
	}
}

func TestResolveModelsFailsForUnknownTier(t *testing.T) {
	definition := Definition{
		Steps: []Step{{
			ID:          "specify",
			Type:        StepCommand,
			CommandName: "speckit.specify",
			ModelType:   "unknown",
		}},
	}
	cfg := domain.ModelConfig{
		DefaultFamily: "openai",
		Families: map[string]domain.ModelFamilyConfig{
			"openai": {Low: "gpt-5-mini"},
		},
	}
	if _, err := ResolveModels(definition, cfg, ""); err == nil {
		t.Fatal("ResolveModels returned nil error, want tier resolution failure")
	}
}
