package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type MockAgent struct{}

func (m *MockAgent) Name() string {
	return "MockAgent"
}

func (m *MockAgent) Design(ctx context.Context, demand string, workspaceDir string, designDir string) error {
	if err := os.MkdirAll(designDir, 0755); err != nil {
		return err
	}

	// Build minimal but structurally valid demand-package.json + solution-design.json
	// so the engine can proceed past validation. Uses stub lane.
	// designDir is like ".dft/runs/<run-id>/design", so run ID is the parent dir name.
	runID := filepath.Base(filepath.Dir(designDir))

	demandPkg := fmt.Sprintf(`{
  "id": "%s",
  "title": "stub demand",
  "raw_demand": %q,
  "refined_demand": "mock refined demand",
  "acceptance_criteria": [
    {"id": "stub-ac-1", "description": "stub acceptance criterion"}
  ],
  "assumptions": ["mock assumption"],
  "non_goals": [],
  "verified_complete": true
}`, runID, demand)

	solution := fmt.Sprintf(`{
  "id": "%s",
  "demand_package_id": "%s",
  "test_plan": {
    "demand_package_id": "%s",
    "requirement_ids": ["stub-req"],
    "scenarios": [
      {
        "id": "stub-scenario",
        "name": "stub scenario",
        "description": "a stub scenario for testing",
        "given": ["a stub test"],
        "when": ["stub runs"],
        "then": ["stub passes"],
        "kind": "e2e"
      }
    ]
  },
  "wbs": {
    "demand_package_id": "%s",
    "specs": [
      {
        "id": "stub-spec",
        "description": "stub spec for testing",
        "prompt_path": "stub",
        "acceptance_criteria": ["stub AC"]
      }
    ]
  },
  "lane_assignments": [
    {
      "spec_id": "stub-spec",
      "lane": "stub",
      "executor_type": "stub"
    }
  ],
  "eval_surface_contract": {
    "demand_package_id": "%s",
    "surfaces": [
      {
        "id": "stub-surface",
        "kind": "topic",
        "artifact_ref": "stub-artifact",
        "adapter_family": "stub",
        "environment_class": "test"
      }
    ]
  }
}`, runID, runID, runID, runID, runID)

	os.WriteFile(filepath.Join(designDir, "demand-package.json"), []byte(demandPkg), 0644)
	return os.WriteFile(filepath.Join(designDir, "solution-design.json"), []byte(solution), 0644)
}
