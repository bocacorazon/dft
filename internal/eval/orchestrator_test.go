package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
)

func TestOrchestratorRunsReadinessAuthorAndExecutor(t *testing.T) {
	root := t.TempDir()
	designDir := filepath.Join(root, ".dft", "runs", "demand-1", "design")
	if err := os.MkdirAll(designDir, 0o755); err != nil {
		t.Fatalf("create design dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(designDir, "wbs.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write WBS artifact: %v", err)
	}
	designWBS := ".dft/runs/demand-1/design/wbs.json"

	result, err := (Orchestrator{
		Agent:        agentstub.Adapter{},
		Verifier:     verify.Checker{RootDir: root},
		ArtifactRoot: root,
		RunID:        "demand-1",
	}).Run(context.Background(), AuthorInput{
		DemandPackage: authorInput().DemandPackage,
		WBS:           authorInput().WBS,
		SurfaceContract: domain.EvalSurfaceContract{
			DemandPackageID: "demand-1",
			Surfaces: []domain.EvalSurface{{
				ID:               "stub-design-artifacts",
				Kind:             domain.EvalSurfaceFile,
				ArtifactRef:      designWBS,
				AdapterFamily:    "file",
				EnvironmentClass: domain.EvalEnvironmentEphemeral,
				Readiness: []domain.ReadinessProbe{{
					ID:   "wbs-exists",
					Kind: domain.ReadinessFileExists,
					Args: []string{designWBS},
				}},
			}},
		},
		ArtifactManifest: domain.ArtifactManifest{
			DemandPackageID: "demand-1",
			Artifacts: []domain.ArtifactRef{{
				ID:   designWBS,
				Kind: domain.ArtifactFile,
				Path: designWBS,
			}},
		},
		StepCatalog: []string{ActionFileExists},
	})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Result.Status != domain.EvalStatusPass {
		t.Fatalf("status = %q, want pass; findings=%v", result.Result.Status, result.Result.Findings)
	}
	for _, path := range []string{
		filepath.Join(root, ".dft", "runs", "demand-1", "eval", "eval-ready.json"),
		filepath.Join(root, ".dft", "runs", "demand-1", "eval", "eval-plan.json"),
		filepath.Join(root, ".dft", "runs", "demand-1", "eval", "evaluation.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected eval artifact %s: %v", path, err)
		}
	}
}
