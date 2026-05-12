package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestStatusInspectCancelAndResumeCommands(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("DFT_RUN_ID", "state-run")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"submit", "--adapter", "stub", "--dry-run", "--dogfood", "Track dogfood runs"}, &stdout, &stderr); code != 0 {
		t.Fatalf("submit returned %d\nstderr: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Fatalf("status returned %d\nstderr: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "state-run") || !strings.Contains(got, "succeeded") {
		t.Fatalf("status output = %q, want run and status", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"inspect", "state-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect returned %d\nstderr: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "evaluation.json") || !strings.Contains(got, "next-demand-package.json") {
		t.Fatalf("inspect output = %q, want artifacts", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"cancel", "state-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("cancel returned %d\nstderr: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"resume", "state-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("resume returned %d\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "state-run") {
		t.Fatalf("resume output = %q, want run id", stdout.String())
	}
}
