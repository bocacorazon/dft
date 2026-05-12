package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunSubmitStubDryRunCreatesDemandPackageArtifacts(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DFT_RUN_ID", "test-run")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--adapter", "stub", "--dry-run", "Build intake loop"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run returned exit code %d, want 0\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "test-run") {
		t.Fatalf("stdout = %q, want run id", stdout.String())
	}

	intentDir := filepath.Join(dir, ".dft", "runs", "test-run", "intent")
	for _, name := range []string{"prompt.md", "stdout.json", "demand-package.json"} {
		if _, err := os.Stat(filepath.Join(intentDir, name)); err != nil {
			t.Fatalf("expected artifact %s: %v", name, err)
		}
	}

	rawPackage, err := os.ReadFile(filepath.Join(intentDir, "demand-package.json"))
	if err != nil {
		t.Fatalf("read demand package: %v", err)
	}

	var demandPackage struct {
		ID                 string   `json:"id"`
		Title              string   `json:"title"`
		RawDemand          string   `json:"raw_demand"`
		AcceptanceCriteria []string `json:"acceptance_criteria"`
	}
	if err := json.Unmarshal(rawPackage, &demandPackage); err != nil {
		t.Fatalf("demand package is invalid JSON: %v\n%s", err, rawPackage)
	}
	if demandPackage.ID != "test-run" {
		t.Fatalf("demand package id = %q, want test-run", demandPackage.ID)
	}
	if demandPackage.RawDemand != "Build intake loop" {
		t.Fatalf("raw demand = %q, want original demand", demandPackage.RawDemand)
	}
	if len(demandPackage.AcceptanceCriteria) == 0 {
		t.Fatalf("acceptance criteria empty, want stub criteria")
	}
}

func TestRunSubmitRejectsUnknownAdapter(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--adapter", "bogus", "--dry-run", "Build intake loop"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("Run returned exit code %d, want 2", code)
	}
	if got := stderr.String(); !strings.Contains(got, `unknown adapter "bogus"`) {
		t.Fatalf("stderr = %q, want adapter error", got)
	}
	if _, err := os.Stat(".dft"); !os.IsNotExist(err) {
		t.Fatalf(".dft exists after rejected submit: %v", err)
	}
}

func TestRunSubmitRequiresDemandText(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--adapter", "stub", "--dry-run"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("Run returned exit code %d, want 2", code)
	}
	if got := stderr.String(); !strings.Contains(got, "submit requires demand text") {
		t.Fatalf("stderr = %q, want missing demand error", got)
	}
}

func TestRunSubmitCopilotAdapterWithoutDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-specific")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("DFT_RUN_ID", "copilot-run")

	binary := filepath.Join(dir, "fake-copilot")
	if err := os.WriteFile(binary, []byte("#!/usr/bin/env sh\ncat >/dev/null\nprintf '{\"id\":\"copilot-run\",\"title\":\"Real Submit\",\"raw_demand\":\"Build real submit\",\"acceptance_criteria\":[\"real adapter selected\"]}\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake copilot: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"submit", "--adapter", "copilot", "--copilot-binary", binary, "Build real submit"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run returned exit code %d, want 0\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "copilot-run") {
		t.Fatalf("stdout = %q, want run id", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".dft", "runs", "copilot-run", "intent", "demand-package.json")); err != nil {
		t.Fatalf("demand package missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".dft", "runs", "copilot-run", "transcripts", "dft-intake.agent.md", "stdout.txt")); err != nil {
		t.Fatalf("copilot transcript missing: %v", err)
	}
}
