package intake

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/ports"
)

func TestCreateDemandPackageRejectsMalformedAgentOutput(t *testing.T) {
	root := t.TempDir()
	service := Service{
		Adapter: staticAgentAdapter{raw: "not-json"},
		RunID:   "bad-run",
		RootDir: root,
	}

	_, err := service.CreateDemandPackage(context.Background(), "Build intake loop")

	if err == nil {
		t.Fatal("CreateDemandPackage returned nil error, want parse failure")
	}
	if !strings.Contains(err.Error(), "parse intake agent output") {
		t.Fatalf("error = %v, want parse context", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".dft", "runs", "bad-run", "intent", "stdout.json")); statErr != nil {
		t.Fatalf("stdout artifact missing: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".dft", "runs", "bad-run", "intent", "demand-package.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("demand-package.json stat error = %v, want not exist", statErr)
	}
}

func TestCreateDemandPackageRejectsInvalidDemandPackage(t *testing.T) {
	root := t.TempDir()
	service := Service{
		Adapter: staticAgentAdapter{raw: `{"id":"bad-run","title":"Bad","raw_demand":"Build intake loop","acceptance_criteria":[]}`},
		RunID:   "bad-run",
		RootDir: root,
	}

	_, err := service.CreateDemandPackage(context.Background(), "Build intake loop")

	if err == nil {
		t.Fatal("CreateDemandPackage returned nil error, want validation failure")
	}
	if !strings.Contains(err.Error(), "validate demand package") {
		t.Fatalf("error = %v, want validation context", err)
	}
}

type staticAgentAdapter struct {
	raw string
}

func (a staticAgentAdapter) Invoke(context.Context, ports.AgentRequest) (ports.AgentResponse, error) {
	return ports.AgentResponse{Raw: a.raw}, nil
}
