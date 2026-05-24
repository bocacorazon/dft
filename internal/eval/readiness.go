package eval

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
)

// ReadinessGate binds declared eval surfaces to delivered artifacts and probes readiness.
type ReadinessGate struct {
	RootDir    string
	HTTPClient *http.Client
}

// Check returns a blocked EvalReady when any surface cannot be bound or probed.
func (g ReadinessGate) Check(ctx context.Context, contract domain.EvalSurfaceContract, manifest domain.ArtifactManifest) (domain.EvalReady, error) {
	if err := contract.Validate(); err != nil {
		return domain.EvalReady{}, fmt.Errorf("validate eval surface contract: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return domain.EvalReady{}, fmt.Errorf("validate artifact manifest: %w", err)
	}
	if contract.DemandPackageID != manifest.DemandPackageID {
		return domain.EvalReady{}, fmt.Errorf("contract demand package %q does not match manifest %q", contract.DemandPackageID, manifest.DemandPackageID)
	}

	artifacts := make(map[string]domain.ArtifactRef, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		artifacts[artifact.ID] = artifact
	}

	ready := domain.EvalReady{Status: domain.EvalStatusPass}
	for _, surface := range contract.Surfaces {
		artifact, ok := artifacts[surface.ArtifactRef]
		if !ok {
			ready.Status = domain.EvalStatusBlocked
			ready.Findings = append(ready.Findings, domain.Finding{
				CheckID:  "surface-binding-" + surface.ID,
				Severity: "high",
				Category: "eval-readiness",
				Message:  fmt.Sprintf("eval surface %q references missing artifact %q", surface.ID, surface.ArtifactRef),
			})
			continue
		}
		binding := domain.SurfaceBinding{
			SurfaceID:  surface.ID,
			ArtifactID: artifact.ID,
			Kind:       artifact.Kind,
			URI:        artifact.URI,
			Path:       artifact.Path,
		}
		ready.Bindings = append(ready.Bindings, binding)
		for _, probe := range surface.Readiness {
			if err := g.runProbe(ctx, binding, probe); err != nil {
				ready.Status = domain.EvalStatusBlocked
				ready.Findings = append(ready.Findings, domain.Finding{
					CheckID:  probe.ID,
					Severity: "high",
					Category: "eval-readiness",
					Message:  err.Error(),
					Location: surface.ID,
				})
			}
		}
	}
	return ready, nil
}

func (g ReadinessGate) runProbe(ctx context.Context, binding domain.SurfaceBinding, probe domain.ReadinessProbe) error {
	switch probe.Kind {
	case domain.ReadinessFileExists:
		if len(probe.Args) != 1 {
			return fmt.Errorf("readiness probe %q requires path argument", probe.ID)
		}
		if _, err := os.Stat(g.path(probe.Args[0])); err != nil {
			return fmt.Errorf("readiness probe %q expected file %s to exist: %v", probe.ID, probe.Args[0], err)
		}
		return nil
	case domain.ReadinessCommandExitZero:
		if len(probe.Args) == 0 {
			return fmt.Errorf("readiness probe %q requires argv", probe.ID)
		}
		cmd := exec.CommandContext(ctx, probe.Args[0], probe.Args[1:]...)
		cmd.Dir = g.root()
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("readiness probe %q command failed: %w: %s", probe.ID, err, strings.TrimSpace(string(output)))
		}
		return nil
	case domain.ReadinessHTTPStatus:
		if len(probe.Args) != 2 {
			return fmt.Errorf("readiness probe %q requires path/url and expected status", probe.ID)
		}
		expected, err := strconv.Atoi(probe.Args[1])
		if err != nil {
			return fmt.Errorf("readiness probe %q expected status must be an integer: %q", probe.ID, probe.Args[1])
		}
		url := probe.Args[0]
		if strings.HasPrefix(url, "/") {
			if binding.URI == "" {
				return fmt.Errorf("readiness probe %q needs bound artifact URI for relative HTTP path", probe.ID)
			}
			url = strings.TrimRight(binding.URI, "/") + url
		}
		client := g.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("readiness probe %q build request: %w", probe.ID, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("readiness probe %q request failed: %w", probe.ID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != expected {
			return fmt.Errorf("readiness probe %q status = %d, want %d", probe.ID, resp.StatusCode, expected)
		}
		return nil
	default:
		return fmt.Errorf("unsupported readiness probe kind %q", probe.Kind)
	}
}

func (g ReadinessGate) path(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(g.root(), path)
}

func (g ReadinessGate) root() string {
	if g.RootDir == "" {
		return "."
	}
	return g.RootDir
}
