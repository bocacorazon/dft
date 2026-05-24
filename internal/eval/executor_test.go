package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
)

func TestExecutorRunsCLIPackAndWritesEvaluationArtifact(t *testing.T) {
	root := t.TempDir()
	writeExecutable(t, filepath.Join(root, "bin", "app"), "#!/bin/sh\necho dft 1.0\n")
	executor := Executor{
		RootDir:  root,
		Verifier: verify.Checker{RootDir: root},
	}

	result, err := executor.Execute(context.Background(), "run-123", readyCLI(), domain.EvalPlan{
		DemandPackageID: "demand-1",
		RequirementIDs:  []string{"REQ-001"},
		Packs: []domain.BDDPack{{
			ID:        "cli-pack",
			SurfaceID: "cli",
			Scenarios: []domain.BDDScenario{{
				ID:             "version",
				Name:           "prints version",
				RequirementIDs: []string{"REQ-001"},
				Steps: []domain.BDDStep{
					{Phase: "when", Action: ActionCLIRun},
					{Phase: "then", Action: ActionCLIExpectExitCode, Args: []string{"0"}},
					{Phase: "then", Action: ActionCLIExpectStdoutContains, Args: []string{"dft 1.0"}},
				},
			}},
		}},
		Checks: []domain.Check{{ID: "app-exists", Kind: domain.CheckFileExists, Args: []string{"bin/app"}}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != domain.EvalStatusPass {
		t.Fatalf("status = %q, want pass; findings=%v", result.Status, result.Findings)
	}
	if result.Coverage.Covered != 1 || result.Coverage.Total != 1 {
		t.Fatalf("coverage = %#v, want 1/1", result.Coverage)
	}
	if len(result.Executions[0].Scenarios[0].Evidence) == 0 {
		t.Fatalf("scenario evidence = 0, want stdout evidence")
	}
	assertEvalArtifact(t, filepath.Join(root, ".dft", "runs", "run-123", "eval", "evaluation.json"))
	if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "eval", "evidence", "version-stdout.txt")); err != nil {
		t.Fatalf("stdout evidence missing: %v", err)
	}
}

func TestExecutorRunsFilePack(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "out.txt"), []byte("artifact ready\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	executor := Executor{RootDir: root}

	result, err := executor.Execute(context.Background(), "", domain.EvalReady{
		Status: domain.EvalStatusPass,
		Bindings: []domain.SurfaceBinding{{
			SurfaceID:  "files",
			ArtifactID: "out",
			Kind:       domain.ArtifactFile,
			Path:       "out.txt",
		}},
	}, domain.EvalPlan{
		DemandPackageID: "demand-1",
		Packs: []domain.BDDPack{{
			ID:        "file-pack",
			SurfaceID: "files",
			Scenarios: []domain.BDDScenario{{
				ID:   "file-content",
				Name: "file contains expected text",
				Steps: []domain.BDDStep{
					{Phase: "then", Action: ActionFileExists, Args: []string{"out.txt"}},
					{Phase: "then", Action: ActionFileContains, Args: []string{"out.txt", "artifact ready"}},
				},
			}},
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != domain.EvalStatusPass {
		t.Fatalf("status = %q, want pass", result.Status)
	}
}

func TestExecutorRunsHTTPPack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	executor := Executor{RootDir: t.TempDir(), HTTPClient: server.Client()}

	result, err := executor.Execute(context.Background(), "", domain.EvalReady{
		Status: domain.EvalStatusPass,
		Bindings: []domain.SurfaceBinding{{
			SurfaceID:  "api",
			ArtifactID: "api-url",
			Kind:       domain.ArtifactURL,
			URI:        server.URL,
		}},
	}, domain.EvalPlan{
		DemandPackageID: "demand-1",
		Packs: []domain.BDDPack{{
			ID:        "api-pack",
			SurfaceID: "api",
			Scenarios: []domain.BDDScenario{{
				ID:   "health",
				Name: "health endpoint is ok",
				Steps: []domain.BDDStep{
					{Phase: "when", Action: ActionHTTPGet, Args: []string{"/health"}},
					{Phase: "then", Action: ActionHTTPExpectStatus, Args: []string{"200"}},
					{Phase: "then", Action: ActionHTTPExpectJSONPath, Args: []string{"status", "ok"}},
				},
			}},
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != domain.EvalStatusPass {
		t.Fatalf("status = %q, want pass; findings=%v", result.Status, result.Findings)
	}
}

func TestExecutorBlocksWhenReadinessBlocked(t *testing.T) {
	result, err := (Executor{RootDir: t.TempDir()}).Execute(context.Background(), "", domain.EvalReady{
		Status: domain.EvalStatusBlocked,
		Findings: []domain.Finding{{
			CheckID: "missing-surface",
			Message: "missing surface",
		}},
	}, domain.EvalPlan{
		DemandPackageID: "demand-1",
		Packs: []domain.BDDPack{{
			ID:        "pack",
			SurfaceID: "cli",
			Scenarios: []domain.BDDScenario{{
				ID:    "scenario",
				Name:  "scenario",
				Steps: []domain.BDDStep{{Action: ActionCLIRun}},
			}},
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != domain.EvalStatusBlocked {
		t.Fatalf("status = %q, want blocked", result.Status)
	}
}

func readyCLI() domain.EvalReady {
	return domain.EvalReady{
		Status: domain.EvalStatusPass,
		Bindings: []domain.SurfaceBinding{{
			SurfaceID:  "cli",
			ArtifactID: "cli-bin",
			Kind:       domain.ArtifactBinary,
			Path:       "bin/app",
		}},
	}
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create executable dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func assertEvalArtifact(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read eval artifact: %v", err)
	}
	var result domain.EvalResult
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("eval artifact invalid JSON: %v\n%s", err, content)
	}
}
