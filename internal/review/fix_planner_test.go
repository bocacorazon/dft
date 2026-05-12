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

func TestFixPlannerWritesWBSAmendmentForFailedEvaluation(t *testing.T) {
	root := t.TempDir()
	planner := FixPlanner{
		Agent:        agentstub.Adapter{},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	amendment, err := planner.Plan(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Fix failed eval",
		RawDemand: "Fix failed eval",
		AcceptanceCriteria: []string{
			"Failures are actionable.",
		},
	}, domain.VerificationResult{
		Status: domain.VerdictFail,
		Findings: []domain.Finding{{
			CheckID: "missing",
			Message: "required artifact missing",
		}},
	})

	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if len(amendment.RemediationSpecs) == 0 {
		t.Fatalf("remediation specs = 0, want at least one")
	}
	content, err := os.ReadFile(filepath.Join(root, ".dft", "runs", "run-123", "fix-plan", "wbs-amendment.json"))
	if err != nil {
		t.Fatalf("read WBS amendment artifact: %v", err)
	}
	var artifact domain.WBSAmendment
	if err := json.Unmarshal(content, &artifact); err != nil {
		t.Fatalf("WBS amendment artifact invalid JSON: %v\n%s", err, content)
	}
}
