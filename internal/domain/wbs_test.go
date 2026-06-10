package domain

import "testing"

func TestWBSValidateAllowsPromptPathWithoutDescription(t *testing.T) {
	wbs := WBS{
		IncrementPackageID: "run-123",
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
		IncrementPackageID: "run-123",
		Specs: []SpecRef{{
			ID:                 "001-missing-input",
			AcceptanceCriteria: []string{"specify writes spec.md"},
		}},
	}

	if err := wbs.Validate(); err == nil {
		t.Fatal("Validate returned nil error, want missing description or prompt_path")
	}
}

func TestWBSValidateRejectsUnknownDependency(t *testing.T) {
	wbs := WBS{
		IncrementPackageID: "run-123",
		Specs: []SpecRef{{
			ID:                 "001-auth",
			Description:        "Build auth",
			DependsOn:          []string{"999-missing"},
			AcceptanceCriteria: []string{"works"},
		}},
	}
	if err := wbs.Validate(); err == nil {
		t.Fatal("Validate returned nil error, want dependency validation failure")
	}
}

func TestWBSValidateAllowsKnownDependencies(t *testing.T) {
	wbs := WBS{
		IncrementPackageID: "run-123",
		BaseBranch:         "main",
		IncrementBranch:    "increment/run-123",
		Specs: []SpecRef{
			{ID: "001-base", Description: "Base", AcceptanceCriteria: []string{"base"}},
			{ID: "002-auth", Description: "Auth", DependsOn: []string{"001-base"}, AcceptanceCriteria: []string{"auth"}},
		},
	}
	if err := wbs.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestWBSValidateRejectsInvalidIncrementBranch(t *testing.T) {
	wbs := WBS{
		IncrementPackageID: "run-123",
		IncrementBranch:    "bad branch",
		Specs: []SpecRef{{
			ID:                 "001-auth",
			Description:        "Build auth",
			AcceptanceCriteria: []string{"works"},
		}},
	}
	if err := wbs.Validate(); err == nil {
		t.Fatal("Validate returned nil error, want invalid increment branch failure")
	}
}
