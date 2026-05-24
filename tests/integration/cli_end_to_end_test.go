package integration_test

import (
	"fmt"
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
	run(t, repo, []string{"DFT_RUN_ID=e2e-run"}, binary, "submit", "--adapter", "stub", "--dry-run", "--full", "Fresh repo end-to-end smoke")
	status := run(t, repo, nil, binary, "status")
	if !strings.Contains(status, "e2e-run") || !strings.Contains(status, "succeeded") {
		t.Fatalf("status output missing run success:\n%s", status)
	}
	inspect := run(t, repo, nil, binary, "inspect", "e2e-run")
	for _, artifact := range []string{"intent/demand-package.json", "eval/eval-plan.json", "review/final-review.json", "macro-result.json"} {
		if !strings.Contains(inspect, artifact) {
			t.Fatalf("inspect output missing %s:\n%s", artifact, inspect)
		}
	}
}

func TestCLIEndToEndStubDogfoodLoopOnFreshDemoBranches(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "dft")
	build := exec.Command("go", "build", "-o", binary, "./cmd/dft")
	build.Dir = "../.."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build dft failed: %v\n%s", err, output)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		repo := filepath.Join(root, fmt.Sprintf("demo-repo-%d", attempt))
		initDemoRepo(t, repo)
		runGit(t, repo, "switch", "-c", fmt.Sprintf("dogfood/attempt-%d", attempt))
		run(t, repo, nil, binary, "init")
		run(t, repo, []string{fmt.Sprintf("DFT_RUN_ID=demo-run-%d", attempt)}, binary, "submit", "--adapter", "stub", "--dry-run", "--dogfood", "Demo project smoke")

		if got := strings.TrimSpace(runGit(t, repo, "branch", "--show-current")); got != fmt.Sprintf("dogfood/attempt-%d", attempt) {
			t.Fatalf("attempt %d current branch = %q", attempt, got)
		}
		runDir := filepath.Join(repo, ".dft", "runs", fmt.Sprintf("demo-run-%d", attempt))
		for _, relative := range []string{
			"design/wbs.json",
			"macro-result.json",
			filepath.Join("specs", "001-demo-project-smoke", "lane-journal.json"),
		} {
			if _, err := os.Stat(filepath.Join(runDir, relative)); err != nil {
				t.Fatalf("attempt %d missing %s: %v", attempt, relative, err)
			}
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

func initDemoRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
	}
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "dft@example.test")
	runGit(t, dir, "config", "user.name", "dft")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("demo project\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "initial")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
