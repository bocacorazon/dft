package orchestration

import (
	"context"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestCompleteIncrementRequiresPassingEvalAndApprovedReview(t *testing.T) {
	git := &recordingGit{defaultBranch: "main"}
	manager := WorktreeManager{Git: git}

	err := manager.CompleteIncrement(context.Background(), CompleteIncrementRequest{
		IncrementBranch: "increment/run-123",
		DefaultBranch:   "main",
		Evaluation:      domain.VerificationResult{Status: domain.VerdictFail},
		Review:          domain.ReviewDecision{Approved: true},
	})
	if err == nil {
		t.Fatal("CompleteIncrement returned nil error, want eval gate failure")
	}

	err = manager.CompleteIncrement(context.Background(), CompleteIncrementRequest{
		IncrementBranch: "increment/run-123",
		DefaultBranch:   "main",
		Evaluation:      domain.VerificationResult{Status: domain.VerdictPass},
		Review:          domain.ReviewDecision{Approved: false},
	})
	if err == nil {
		t.Fatal("CompleteIncrement returned nil error, want review gate failure")
	}
}

func TestCompleteIncrementMergesApprovedIncrementToDefault(t *testing.T) {
	git := &recordingGit{defaultBranch: "main"}
	manager := WorktreeManager{Git: git}

	err := manager.CompleteIncrement(context.Background(), CompleteIncrementRequest{
		IncrementBranch: "increment/run-123",
		DefaultBranch:   "main",
		Evaluation:      domain.VerificationResult{Status: domain.VerdictPass},
		Review:          domain.ReviewDecision{Approved: true},
	})

	if err != nil {
		t.Fatalf("CompleteIncrement returned error: %v", err)
	}
	want := ports.MergeRequest{Source: "increment/run-123", Target: "main"}
	if git.merged != want {
		t.Fatalf("merge = %#v, want %#v", git.merged, want)
	}
}
