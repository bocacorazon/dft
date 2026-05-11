package gitx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	BaseBranch string
	Branch     string
	Path       string
}

func CreateWorktree(repoRoot string, runID string) (Worktree, error) {
	baseBranch, err := currentBranch(repoRoot)
	if err != nil {
		return Worktree{}, err
	}

	branch := fmt.Sprintf("dft-%s", runID)
	path := filepath.Join(repoRoot, ".dft", "worktrees", runID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Worktree{}, fmt.Errorf("create worktrees dir: %w", err)
	}

	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "-b", branch, path, baseBranch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Worktree{}, fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return Worktree{
		BaseBranch: baseBranch,
		Branch:     branch,
		Path:       path,
	}, nil
}

func MergeAndCleanup(repoRoot string, wt Worktree) error {
	if err := gitRun(repoRoot, "checkout", wt.BaseBranch); err != nil {
		return err
	}
	if err := gitRun(repoRoot, "merge", "--no-ff", "--no-edit", wt.Branch); err != nil {
		return err
	}
	if err := gitRun(repoRoot, "worktree", "remove", wt.Path, "--force"); err != nil {
		return err
	}
	if err := gitRun(repoRoot, "branch", "-D", wt.Branch); err != nil {
		return err
	}
	return nil
}

func ExistingWorktree(repoRoot string, runID string) (Worktree, error) {
	path := filepath.Join(repoRoot, ".dft", "worktrees", runID)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Worktree{}, fmt.Errorf("worktree not found for run %s", runID)
		}
		return Worktree{}, fmt.Errorf("stat worktree: %w", err)
	}
	branchOut, err := gitOutput(path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return Worktree{}, err
	}
	branch := strings.TrimSpace(branchOut)
	baseBranchOut, err := gitOutput(repoRoot, "for-each-ref", "--format=%(refname:short)", "--contains", branch, "refs/heads")
	if err != nil {
		return Worktree{}, err
	}
	baseBranch := currentBranchFallback(strings.TrimSpace(baseBranchOut), repoRoot)
	return Worktree{
		BaseBranch: baseBranch,
		Branch:     branch,
		Path:       path,
	}, nil
}

func CommittedStepIDs(worktreePath string, runID string) (map[string]bool, error) {
	out, err := gitOutput(worktreePath, "log", "--format=%B%n---")
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool)
	entries := strings.Split(out, "\n---\n")
	for _, entry := range entries {
		lines := strings.Split(entry, "\n")
		var commitRunID string
		var stepID string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "run-id:") {
				commitRunID = strings.TrimSpace(strings.TrimPrefix(line, "run-id:"))
			}
			if strings.HasPrefix(strings.ToLower(line), "step-id:") {
				stepID = strings.TrimSpace(strings.TrimPrefix(line, "step-id:"))
			}
		}
		if commitRunID == runID && stepID != "" {
			ids[stepID] = true
		}
	}
	return ids, nil
}

func CommitIfDirty(worktreePath string, runID string, stepID string) (bool, error) {
	status, err := gitOutput(worktreePath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(status) == "" {
		return false, nil
	}

	if err := gitRun(worktreePath, "add", "-A"); err != nil {
		return false, err
	}

	message := fmt.Sprintf("dft: execute step %s", stepID)
	trailer := fmt.Sprintf("run-id: %s\nstep-id: %s", runID, stepID)
	if err := gitRun(worktreePath, "commit", "-m", message, "-m", trailer); err != nil {
		return false, err
	}
	return true, nil
}

func currentBranch(repoRoot string) (string, error) {
	out, err := gitOutput(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		return "", fmt.Errorf("could not determine current branch")
	}
	return branch, nil
}

func currentBranchFallback(candidates string, repoRoot string) string {
	for _, line := range strings.Split(candidates, "\n") {
		c := strings.TrimSpace(line)
		if c != "" && !strings.HasPrefix(c, "dft-run-") {
			return c
		}
	}
	branch, err := currentBranch(repoRoot)
	if err != nil {
		return "main"
	}
	return branch
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
