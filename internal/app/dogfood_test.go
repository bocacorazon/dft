package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSubmitFullStubCreatesFullProcessWithoutDogfoodArtifacts(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("DFT_RUN_ID", "full-run")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--adapter", "stub", "--dry-run", "--full", "Improve dft status output"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run returned exit code %d, want 0\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "full process complete") {
		t.Fatalf("stdout = %q, want full process completion", stdout.String())
	}

	runDir := filepath.Join(root, ".dft", "runs", "full-run")
	for _, relative := range []string{
		"intent/demand-package.json",
		"design/wbs.json",
		"design/lane-assignments.json",
		"design/eval-surfaces.json",
		"eval/eval-ready.json",
		"eval/eval-plan.json",
		"eval/evaluation.json",
		"macro-result.json",
		"review/final-review.json",
	} {
		path := filepath.Join(runDir, relative)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", relative, err)
		}
	}
	for _, relative := range []string{
		"steps/dogfood-intake/parsed.json",
		"dogfood-feedback-evaluation.json",
		"next-demand-package.json",
	} {
		if _, err := os.Stat(filepath.Join(runDir, relative)); !os.IsNotExist(err) {
			t.Fatalf("dogfood-only artifact %s exists after --full: %v", relative, err)
		}
	}
}

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
		"design/eval-surfaces.json",
		"steps/dogfood-intake/parsed.json",
		"eval/eval-ready.json",
		"eval/eval-plan.json",
		"eval/evaluation.json",
		"dogfood-feedback-evaluation.json",
		"macro-result.json",
		"review/final-review.json",
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

func TestRunSubmitDogfoodCanHoldIncrement(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("DFT_RUN_ID", "held-run")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--adapter", "stub", "--dry-run", "--dogfood", "--hold-increment", "Improve dft status output"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run returned exit code %d, want 0\nstderr: %s", code, stderr.String())
	}
	content, err := os.ReadFile(filepath.Join(root, ".dft", "runs", "held-run", "macro-result.json"))
	if err != nil {
		t.Fatalf("read macro result: %v", err)
	}
	if !strings.Contains(string(content), `"increment_held": true`) {
		t.Fatalf("macro result missing held increment marker:\n%s", content)
	}
}
