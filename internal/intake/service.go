package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// Service creates demand packages through an agent adapter.
type Service struct {
	Adapter ports.AgentAdapter
	RunID   string
	RootDir string
}

// CreateDemandPackage writes prompt, raw output, and parsed package artifacts.
func (s Service) CreateDemandPackage(ctx context.Context, demand string) (domain.DemandPackage, error) {
	demand = strings.TrimSpace(demand)
	if demand == "" {
		return domain.DemandPackage{}, fmt.Errorf("submit requires demand text")
	}
	if s.Adapter == nil {
		return domain.DemandPackage{}, fmt.Errorf("agent adapter is required")
	}
	if s.RunID == "" {
		return domain.DemandPackage{}, fmt.Errorf("run id is required")
	}

	intentDir := filepath.Join(s.RootDir, ".dft", "runs", s.RunID, "intent")
	if err := os.MkdirAll(intentDir, 0o755); err != nil {
		return domain.DemandPackage{}, fmt.Errorf("create intent artifact directory: %w", err)
	}

	prompt := "# dft-intake\n\nNormalize this demand into a demand package.\n\nRun ID:\n" + s.RunID + "\n\nDemand:\n" + demand + "\n"
	if err := os.WriteFile(filepath.Join(intentDir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		return domain.DemandPackage{}, fmt.Errorf("write prompt artifact: %w", err)
	}

	response, err := s.Adapter.Invoke(ctx, ports.AgentRequest{
		AgentName: "dft-intake.agent.md",
		Prompt:    prompt,
		Demand:    demand,
		RunID:     s.RunID,
	})
	if err != nil {
		return domain.DemandPackage{}, fmt.Errorf("invoke intake agent: %w", err)
	}
	if err := os.WriteFile(filepath.Join(intentDir, "stdout.json"), []byte(response.Raw), 0o644); err != nil {
		return domain.DemandPackage{}, fmt.Errorf("write agent output artifact: %w", err)
	}

	var demandPackage domain.DemandPackage
	if err := json.Unmarshal([]byte(response.Raw), &demandPackage); err != nil {
		return domain.DemandPackage{}, fmt.Errorf("parse intake agent output: %w", err)
	}
	if demandPackage.ID == "" || demandPackage.ID == "unknown" {
		demandPackage.ID = s.RunID
	}
	if strings.TrimSpace(demandPackage.RawDemand) == "" {
		demandPackage.RawDemand = demand
	}
	if err := demandPackage.Validate(); err != nil {
		return domain.DemandPackage{}, fmt.Errorf("validate demand package: %w", err)
	}

	parsed, err := json.MarshalIndent(demandPackage, "", "  ")
	if err != nil {
		return domain.DemandPackage{}, fmt.Errorf("encode demand package: %w", err)
	}
	if err := os.WriteFile(filepath.Join(intentDir, "demand-package.json"), append(parsed, '\n'), 0o644); err != nil {
		return domain.DemandPackage{}, fmt.Errorf("write demand package artifact: %w", err)
	}

	return demandPackage, nil
}
