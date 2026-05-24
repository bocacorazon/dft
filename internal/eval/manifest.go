package eval

import (
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
)

// ManifestFromSurfaceContract creates the first local artifact manifest from declared surfaces.
func ManifestFromSurfaceContract(demandPackageID string, contract domain.EvalSurfaceContract) domain.ArtifactManifest {
	manifest := domain.ArtifactManifest{DemandPackageID: demandPackageID}
	seen := map[string]struct{}{}
	for _, surface := range contract.Surfaces {
		if _, ok := seen[surface.ArtifactRef]; ok {
			continue
		}
		seen[surface.ArtifactRef] = struct{}{}
		artifact := domain.ArtifactRef{
			ID:   surface.ArtifactRef,
			Kind: artifactKindForSurface(surface),
		}
		if strings.HasPrefix(surface.ArtifactRef, "http://") || strings.HasPrefix(surface.ArtifactRef, "https://") {
			artifact.URI = surface.ArtifactRef
		} else {
			artifact.Path = surface.ArtifactRef
		}
		manifest.Artifacts = append(manifest.Artifacts, artifact)
	}
	return manifest
}

func artifactKindForSurface(surface domain.EvalSurface) domain.ArtifactKind {
	switch surface.Kind {
	case domain.EvalSurfaceCLI:
		return domain.ArtifactBinary
	case domain.EvalSurfaceHTTPAPI, domain.EvalSurfaceGraphQL, domain.EvalSurfaceGRPC, domain.EvalSurfaceWebUI:
		return domain.ArtifactURL
	case domain.EvalSurfaceContainer:
		return domain.ArtifactImage
	case domain.EvalSurfaceEvent:
		return domain.ArtifactTopic
	case domain.EvalSurfaceFile, domain.EvalSurfaceInfra, domain.EvalSurfaceDatabase, domain.EvalSurfaceComposite:
		return domain.ArtifactFile
	default:
		return domain.ArtifactFile
	}
}

// DefaultStepCatalog lists the first artifact-only BDD actions available to eval authors.
func DefaultStepCatalog() []string {
	return []string{
		ActionCLIRun,
		ActionCLIExpectExitCode,
		ActionCLIExpectStdoutContains,
		ActionCLIExpectStderrContains,
		ActionFileExists,
		ActionFileContains,
		ActionHTTPGet,
		ActionHTTPPostJSON,
		ActionHTTPExpectStatus,
		ActionHTTPExpectJSONPath,
	}
}
