package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/domain"
)

func TestEvalPlanAuthorWritesPlanArtifact(t *testing.T) {
	root := t.TempDir()
	author := EvalPlanAuthor{
		Agent:        agentstub.Adapter{},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	plan, err := author.Author(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Evaluate output",
		RawDemand: "Evaluate output",
		AcceptanceCriteria: []string{
			"Evaluation plan is deterministic.",
		},
	})

	if err != nil {
		t.Fatalf("Author returned error: %v", err)
	}
	if len(plan.Checks) == 0 {
		t.Fatalf("plan checks = 0, want at least one")
	}
	content, err := os.ReadFile(filepath.Join(root, ".dft", "runs", "run-123", "eval-plan.json"))
	if err != nil {
		t.Fatalf("read eval plan artifact: %v", err)
	}
	var artifact domain.EvaluationPlan
	if err := json.Unmarshal(content, &artifact); err != nil {
		t.Fatalf("eval plan artifact invalid JSON: %v\n%s", err, content)
	}
}
