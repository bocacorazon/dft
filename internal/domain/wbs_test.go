package domain

import "testing"

func TestWBSValidateAllowsPromptPathWithoutDescription(t *testing.T) {
	wbs := WBS{
		DemandPackageID: "run-123",
		Specs: []SpecRef{{
			ID:                 "001-authored-prompt",
			PromptPath:         "docs/specs/001-authored-prompt/prompt.md",
			AcceptanceCriteria: []string{"specify writes spec.md"},
		}},
	}

	if err := wbs.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestWBSValidateRequiresDescriptionOrPromptPath(t *testing.T) {
	wbs := WBS{
		DemandPackageID: "run-123",
		Specs: []SpecRef{{
			ID:                 "001-missing-input",
			AcceptanceCriteria: []string{"specify writes spec.md"},
		}},
	}

	if err := wbs.Validate(); err == nil {
		t.Fatal("Validate returned nil error, want missing description or prompt_path")
	}
}
