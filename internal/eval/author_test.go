package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestArtifactOnlyPlanAuthorWritesHiddenEvalPlan(t *testing.T) {
	root := t.TempDir()
	agent := captureAgent{plan: domain.EvalPlan{
		DemandPackageID: "demand-1",
		RequirementIDs:  []string{"REQ-001"},
		Packs: []domain.BDDPack{{
			ID:        "cli-pack",
			SurfaceID: "cli",
			Scenarios: []domain.BDDScenario{{
				ID:             "version",
				Name:           "prints version",
				RequirementIDs: []string{"REQ-001"},
				Steps:          []domain.BDDStep{{Action: ActionCLIRun}},
			}},
		}},
	}}
	author := ArtifactOnlyPlanAuthor{
		Agent:        &agent,
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	plan, err := author.Author(context.Background(), authorInput())

	if err != nil {
		t.Fatalf("Author returned error: %v", err)
	}
	if plan.Packs[0].Visibility != domain.EvalVisibilityHidden {
		t.Fatalf("visibility = %q, want hidden", plan.Packs[0].Visibility)
	}
	if !strings.Contains(agent.prompt, "surface_contract") {
		t.Fatalf("prompt does not include surface contract: %s", agent.prompt)
	}
	assertEvalPlanArtifact(t, filepath.Join(root, ".dft", "runs", "run-123", "eval", "eval-plan.json"))
}

type captureAgent struct {
	plan   domain.EvalPlan
	prompt string
}

func (a *captureAgent) Invoke(_ context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	a.prompt = request.Prompt
	content, err := json.Marshal(a.plan)
	if err != nil {
		return ports.AgentResponse{}, err
	}
	return ports.AgentResponse{Raw: string(content)}, nil
}

func authorInput() AuthorInput {
	return AuthorInput{
		DemandPackage: domain.DemandPackage{
			ID:        "demand-1",
			Title:     "Evaluate CLI",
			RawDemand: "Build a CLI",
			AcceptanceCriteria: []string{
				"CLI prints version",
			},
		},
		WBS: domain.WBS{
			DemandPackageID: "demand-1",
			Specs: []domain.SpecRef{{
				ID:                 "001-cli",
				Description:        "Build CLI",
				AcceptanceCriteria: []string{"CLI prints version"},
			}},
		},
		SurfaceContract: evalSurfaceContract(),
		ArtifactManifest: domain.ArtifactManifest{
			DemandPackageID: "demand-1",
			Artifacts: []domain.ArtifactRef{{
				ID:   "cli-bin",
				Kind: domain.ArtifactBinary,
				Path: "bin/app",
			}},
		},
		Readiness: readyCLI(),
		StepCatalog: []string{
			ActionCLIRun,
			ActionCLIExpectExitCode,
		},
	}
}

func assertEvalPlanArtifact(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read eval plan artifact: %v", err)
	}
	var plan domain.EvalPlan
	if err := json.Unmarshal(content, &plan); err != nil {
		t.Fatalf("eval plan artifact invalid JSON: %v\n%s", err, content)
	}
}
