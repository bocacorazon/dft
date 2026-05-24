package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestSurfaceContractAuthorWritesDesignArtifact(t *testing.T) {
	root := t.TempDir()
	agent := surfaceAgent{contract: evalSurfaceContract()}
	author := SurfaceContractAuthor{
		Agent:        &agent,
		ArtifactRoot: root,
		RunID:        "demand-1",
	}

	contract, err := author.Author(context.Background(), authorInput().DemandPackage, authorInput().WBS)

	if err != nil {
		t.Fatalf("Author returned error: %v", err)
	}
	if len(contract.Surfaces) != 1 || contract.Surfaces[0].ID != "cli" {
		t.Fatalf("contract = %#v, want cli surface", contract)
	}
	assertSurfaceContractArtifact(t, filepath.Join(root, ".dft", "runs", "demand-1", "design", "eval-surfaces.json"))
}

type surfaceAgent struct {
	contract domain.EvalSurfaceContract
}

func (a *surfaceAgent) Invoke(_ context.Context, _ ports.AgentRequest) (ports.AgentResponse, error) {
	content, err := json.Marshal(a.contract)
	if err != nil {
		return ports.AgentResponse{}, err
	}
	return ports.AgentResponse{Raw: string(content)}, nil
}

func assertSurfaceContractArtifact(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read surface contract artifact: %v", err)
	}
	var contract domain.EvalSurfaceContract
	if err := json.Unmarshal(content, &contract); err != nil {
		t.Fatalf("surface contract artifact invalid JSON: %v\n%s", err, content)
	}
}
