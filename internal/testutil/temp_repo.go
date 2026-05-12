// Package testutil contains helpers for integration-style tests.
package testutil

import (
	"os/exec"
	"testing"
)

// TempGitRepo creates an empty git repository for tests and returns its path.
func TempGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "dft@example.invalid")
	runGit(t, dir, "config", "user.name", "DFT Tests")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
