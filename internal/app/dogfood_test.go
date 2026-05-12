package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSubmitDogfoodStubCreatesFullFeedbackLoopArtifacts(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("DFT_RUN_ID", "dogfood-run")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--adapter", "stub", "--dry-run", "--dogfood", "Improve dft status output"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run returned exit code %d, want 0\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "dogfood loop complete") {
		t.Fatalf("stdout = %q, want dogfood completion", stdout.String())
	}

	runDir := filepath.Join(root, ".dft", "runs", "dogfood-run")
	for _, relative := range []string{
		"intent/demand-package.json",
		"design/wbs.json",
		"design/lane-assignments.json",
		"steps/dogfood-intake/parsed.json",
		"evaluation.json",
		"macro-result.json",
		"next-demand-package.json",
	} {
		path := filepath.Join(runDir, relative)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", relative, err)
		}
	}

	content, err := os.ReadFile(filepath.Join(runDir, "next-demand-package.json"))
	if err != nil {
		t.Fatalf("read next demand package: %v", err)
	}
	var next struct {
		RawDemand string `json:"raw_demand"`
	}
	if err := json.Unmarshal(content, &next); err != nil {
		t.Fatalf("next demand package invalid JSON: %v\n%s", err, content)
	}
	if !strings.Contains(next.RawDemand, "Improve dft status output") {
		t.Fatalf("next raw demand = %q, want original request context", next.RawDemand)
	}
}
