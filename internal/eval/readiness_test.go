package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestReadinessGateBindsArtifactAndRunsProbe(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "app"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}
	gate := ReadinessGate{RootDir: root}

	ready, err := gate.Check(context.Background(), evalSurfaceContract(), domain.ArtifactManifest{
		DemandPackageID: "demand-1",
		Artifacts: []domain.ArtifactRef{{
			ID:   "cli-bin",
			Kind: domain.ArtifactBinary,
			Path: "bin/app",
		}},
	})

	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if ready.Status != domain.EvalStatusPass {
		t.Fatalf("status = %q, want pass; findings=%v", ready.Status, ready.Findings)
	}
	if len(ready.Bindings) != 1 || ready.Bindings[0].SurfaceID != "cli" {
		t.Fatalf("bindings = %#v, want cli binding", ready.Bindings)
	}
}

func TestReadinessGateBlocksMissingArtifact(t *testing.T) {
	gate := ReadinessGate{RootDir: t.TempDir()}

	ready, err := gate.Check(context.Background(), evalSurfaceContract(), domain.ArtifactManifest{
		DemandPackageID: "demand-1",
		Artifacts: []domain.ArtifactRef{{
			ID:   "other",
			Kind: domain.ArtifactBinary,
			Path: "bin/other",
		}},
	})

	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if ready.Status != domain.EvalStatusBlocked {
		t.Fatalf("status = %q, want blocked", ready.Status)
	}
	if len(ready.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(ready.Findings))
	}
}

func evalSurfaceContract() domain.EvalSurfaceContract {
	return domain.EvalSurfaceContract{
		DemandPackageID: "demand-1",
		Surfaces: []domain.EvalSurface{{
			ID:               "cli",
			Kind:             domain.EvalSurfaceCLI,
			ArtifactRef:      "cli-bin",
			AdapterFamily:    "cli",
			EnvironmentClass: domain.EvalEnvironmentEphemeral,
			Readiness: []domain.ReadinessProbe{{
				ID:   "binary-exists",
				Kind: domain.ReadinessFileExists,
				Args: []string{"bin/app"},
			}},
		}},
	}
}
