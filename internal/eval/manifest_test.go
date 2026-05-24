package eval

import (
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestManifestFromSurfaceContractCollectsUniqueArtifacts(t *testing.T) {
	manifest := ManifestFromSurfaceContract("demand-1", domain.EvalSurfaceContract{
		DemandPackageID: "demand-1",
		Surfaces: []domain.EvalSurface{
			{ID: "cli", Kind: domain.EvalSurfaceCLI, ArtifactRef: "bin/app"},
			{ID: "cli-smoke", Kind: domain.EvalSurfaceCLI, ArtifactRef: "bin/app"},
			{ID: "api", Kind: domain.EvalSurfaceHTTPAPI, ArtifactRef: "https://example.test"},
		},
	})

	if len(manifest.Artifacts) != 2 {
		t.Fatalf("artifact count = %d, want 2", len(manifest.Artifacts))
	}
	if manifest.Artifacts[0].Kind != domain.ArtifactBinary || manifest.Artifacts[0].Path != "bin/app" {
		t.Fatalf("first artifact = %#v, want binary path", manifest.Artifacts[0])
	}
	if manifest.Artifacts[1].Kind != domain.ArtifactURL || manifest.Artifacts[1].URI != "https://example.test" {
		t.Fatalf("second artifact = %#v, want URL", manifest.Artifacts[1])
	}
}
