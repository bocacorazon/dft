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

func TestFinalReviewerWritesApprovedDecision(t *testing.T) {
	root := t.TempDir()
	reviewer := FinalReviewer{
		Agent:        agentstub.Adapter{},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	decision, err := reviewer.Review(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Review increment",
		RawDemand: "Review increment",
		AcceptanceCriteria: []string{
			"Review is required.",
		},
	}, "increment/run-123")

	if err != nil {
		t.Fatalf("Review returned error: %v", err)
	}
	if !decision.Approved {
		t.Fatalf("approved = false, want true")
	}
	content, err := os.ReadFile(filepath.Join(root, ".dft", "runs", "run-123", "review", "final-review.json"))
	if err != nil {
		t.Fatalf("read final review artifact: %v", err)
	}
	var artifact domain.ReviewDecision
	if err := json.Unmarshal(content, &artifact); err != nil {
		t.Fatalf("final review artifact invalid JSON: %v\n%s", err, content)
	}
}
