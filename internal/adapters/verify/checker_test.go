package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
)

func TestCheckerEvaluatesFileAndGrepChecks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "result.txt"), []byte("hello dft\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	checker := Checker{RootDir: root}
	result := checker.Run(context.Background(), []domain.Check{
		{ID: "file", Kind: domain.CheckFileExists, Args: []string{"result.txt"}},
		{ID: "grep", Kind: domain.CheckGrepMatches, Args: []string{"result.txt", "hello dft"}},
	})

	if result.Status != domain.VerdictPass {
		t.Fatalf("status = %q, want pass; findings=%#v", result.Status, result.Findings)
	}
	if len(result.Results) != 2 {
		t.Fatalf("check result count = %d, want 2", len(result.Results))
	}
}

func TestCheckerReportsFailureFindings(t *testing.T) {
	checker := Checker{RootDir: t.TempDir()}
	result := checker.Run(context.Background(), []domain.Check{
		{ID: "missing", Kind: domain.CheckFileExists, Args: []string{"missing.txt"}},
	})

	if result.Status != domain.VerdictFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("finding count = %d, want 1", len(result.Findings))
	}
	if result.Findings[0].CheckID != "missing" {
		t.Fatalf("finding check id = %q, want missing", result.Findings[0].CheckID)
	}
}

func TestCheckerRunsArgvCommandChecks(t *testing.T) {
	checker := Checker{RootDir: t.TempDir()}
	result := checker.Run(context.Background(), []domain.Check{
		{ID: "command", Kind: domain.CheckCommandExitZero, Args: []string{"git", "--version"}},
	})

	if result.Status != domain.VerdictPass {
		t.Fatalf("status = %q, want pass; findings=%#v", result.Status, result.Findings)
	}
}

func TestCheckerEvaluatesJSONPathEquals(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "result.json"), []byte(`{"status":"pass","nested":{"count":2}}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	checker := Checker{RootDir: root}
	result := checker.Run(context.Background(), []domain.Check{
		{ID: "status", Kind: domain.CheckJSONPathEquals, Args: []string{"result.json", "status", "pass"}},
		{ID: "count", Kind: domain.CheckJSONPathEquals, Args: []string{"result.json", "nested.count", "2"}},
	})

	if result.Status != domain.VerdictPass {
		t.Fatalf("status = %q, want pass; findings=%#v", result.Status, result.Findings)
	}
}

func TestCheckerEvaluatesCountMatchesAtLeastAndOS(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "log.txt"), []byte("pass\nfail\npass\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	checker := Checker{RootDir: root}
	result := checker.Run(context.Background(), []domain.Check{
		{ID: "count", Kind: domain.CheckCountMatchesAtLeast, Args: []string{"log.txt", "pass", "2"}},
		{ID: "os", Kind: domain.CheckOS, Args: []string{runtime.GOOS}},
	})

	if result.Status != domain.VerdictPass {
		t.Fatalf("status = %q, want pass; findings=%#v", result.Status, result.Findings)
	}
}

func TestCheckerReportsCountMatchesAtLeastFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "log.txt"), []byte("pass\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	checker := Checker{RootDir: root}
	result := checker.Run(context.Background(), []domain.Check{
		{ID: "count", Kind: domain.CheckCountMatchesAtLeast, Args: []string{"log.txt", "pass", "2"}},
	})

	if result.Status != domain.VerdictFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if got := result.Findings[0].CheckID; got != "count" {
		t.Fatalf("finding check id = %q, want count", got)
	}
}

func TestCheckerRejectsTrackedBinaryArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app"), []byte{0x7f, 'E', 'L', 'F', 0x00}, 0o755); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}
	runGit(t, root, "init")
	runGit(t, root, "add", "app")

	result := (Checker{RootDir: root}).Run(context.Background(), []domain.Check{
		{ID: "no-binaries", Kind: domain.CheckNoBinaryArtifacts},
	})

	if result.Status != domain.VerdictFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if got := result.Findings[0].Message; !strings.Contains(got, "app") {
		t.Fatalf("finding message = %q, want binary path", got)
	}
}

func TestCheckerAllowsTrackedSourceFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	runGit(t, root, "init")
	runGit(t, root, "add", "main.go")

	result := (Checker{RootDir: root}).Run(context.Background(), []domain.Check{
		{ID: "no-binaries", Kind: domain.CheckNoBinaryArtifacts},
	})

	if result.Status != domain.VerdictPass {
		t.Fatalf("status = %q, want pass; findings=%#v", result.Status, result.Findings)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
