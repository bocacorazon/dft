package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
)

func TestEvaluatorWritesPassingEvaluationArtifact(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "done.txt"), []byte("done\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	evaluator := Evaluator{
		Verifier:     verify.Checker{RootDir: root},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	result, err := evaluator.Evaluate(context.Background(), []domain.Check{
		{ID: "done", Kind: domain.CheckFileExists, Args: []string{"done.txt"}},
	})

	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if result.Status != domain.VerdictPass {
		t.Fatalf("status = %q, want pass", result.Status)
	}
	assertEvaluationArtifact(t, filepath.Join(root, ".dft", "runs", "run-123", "evaluation.json"))
}

func TestEvaluatorReturnsFailedVerdictWithFindings(t *testing.T) {
	root := t.TempDir()
	evaluator := Evaluator{
		Verifier:     verify.Checker{RootDir: root},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	result, err := evaluator.Evaluate(context.Background(), []domain.Check{
		{ID: "missing", Kind: domain.CheckFileExists, Args: []string{"missing.txt"}},
	})

	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if result.Status != domain.VerdictFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("finding count = %d, want 1", len(result.Findings))
	}
	assertEvaluationArtifact(t, filepath.Join(root, ".dft", "runs", "run-123", "evaluation.json"))
}

func assertEvaluationArtifact(t *testing.T, path string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evaluation artifact: %v", err)
	}
	var result domain.VerificationResult
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("evaluation artifact invalid JSON: %v\n%s", err, content)
	}
}
