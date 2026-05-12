package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIEndToEndStubRunInFreshRepo(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "dft")
	build := exec.Command("go", "build", "-o", binary, "./cmd/dft")
	build.Dir = "../.."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build dft failed: %v\n%s", err, output)
	}

	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
	}
	run(t, repo, nil, binary, "init")
	run(t, repo, []string{"DFT_RUN_ID=e2e-run"}, binary, "submit", "--adapter", "stub", "--dry-run", "--dogfood", "Fresh repo end-to-end smoke")
	status := run(t, repo, nil, binary, "status")
	if !strings.Contains(status, "e2e-run") || !strings.Contains(status, "succeeded") {
		t.Fatalf("status output missing run success:\n%s", status)
	}
	inspect := run(t, repo, nil, binary, "inspect", "e2e-run")
	for _, artifact := range []string{"intent/demand-package.json", "eval-plan.json", "review/final-review.json", "macro-result.json"} {
		if !strings.Contains(inspect, artifact) {
			t.Fatalf("inspect output missing %s:\n%s", artifact, inspect)
		}
	}
}

func run(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
	return string(output)
}
