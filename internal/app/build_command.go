package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
)

func loadSolutionDesign(runID string) (domain.SolutionDesign, error) {
	path := filepath.Join(".dft", "runs", runID, "design", "solution-design.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return domain.SolutionDesign{}, fmt.Errorf("read solution design: %w (run the solution phase first)", err)
	}
	var sd domain.SolutionDesign
	if err := json.Unmarshal(content, &sd); err != nil {
		return domain.SolutionDesign{}, fmt.Errorf("parse solution design: %w", err)
	}
	if err := sd.Validate(); err != nil {
		return domain.SolutionDesign{}, fmt.Errorf("invalid solution design: %w", err)
	}
	return sd, nil
}

func loadOrchestrationResult(runID string) (domain.OrchestrationResult, error) {
	path := filepath.Join(".dft", "runs", runID, "orchestration-result.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return domain.OrchestrationResult{}, fmt.Errorf("read orchestration result: %w", err)
	}
	var result domain.OrchestrationResult
	if err := json.Unmarshal(content, &result); err != nil {
		return domain.OrchestrationResult{}, fmt.Errorf("parse orchestration result: %w", err)
	}
	return result, nil
}
