package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// Evaluator runs deterministic checks and persists the feedback artifact.
type Evaluator struct {
	Verifier     ports.Verifier
	ArtifactRoot string
	RunID        string
}

// Evaluate executes checks and writes `.dft/runs/<run-id>/evaluation.json`.
func (e Evaluator) Evaluate(ctx context.Context, checks []domain.Check) (domain.VerificationResult, error) {
	if e.Verifier == nil {
		return domain.VerificationResult{}, fmt.Errorf("verifier is required")
	}
	if e.RunID == "" {
		return domain.VerificationResult{}, fmt.Errorf("run id is required")
	}

	result := e.Verifier.Run(ctx, checks)
	path := filepath.Join(e.ArtifactRoot, ".dft", "runs", e.RunID, "evaluation.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.VerificationResult{}, fmt.Errorf("create evaluation artifact directory: %w", err)
	}

	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return domain.VerificationResult{}, fmt.Errorf("encode evaluation artifact: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return domain.VerificationResult{}, fmt.Errorf("write evaluation artifact: %w", err)
	}

	return result, nil
}
