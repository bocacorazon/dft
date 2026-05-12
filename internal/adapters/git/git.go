package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bocacorazon/dft/internal/ports"
)

// Adapter implements git operations with git's argv-based CLI.
type Adapter struct {
	RepoDir string
}

// DefaultBranch resolves the remote default branch when available, otherwise the current branch.
func (a Adapter) DefaultBranch(ctx context.Context) (string, error) {
	remoteHead, err := a.run(ctx, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		branch := strings.TrimPrefix(strings.TrimSpace(remoteHead), "origin/")
		if branch != "" {
			return branch, nil
		}
	}

	current, err := a.run(ctx, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(current)
	if branch == "" {
		return "", fmt.Errorf("current branch is empty")
	}
	return branch, nil
}

// CreateBranch creates a branch from the provided base without switching worktrees.
func (a Adapter) CreateBranch(ctx context.Context, request ports.CreateBranchRequest) error {
	_, err := a.run(ctx, "branch", request.Name, request.Base)
	return err
}

// CreateWorktree creates a worktree with a new branch from the provided base.
func (a Adapter) CreateWorktree(ctx context.Context, request ports.CreateWorktreeRequest) error {
	_, err := a.run(ctx, "worktree", "add", "-b", request.Branch, request.Path, request.Base)
	return err
}

// Merge checks out the target branch in the repository worktree and merges source into it.
func (a Adapter) Merge(ctx context.Context, request ports.MergeRequest) error {
	if _, err := a.run(ctx, "checkout", request.Target); err != nil {
		return err
	}
	_, err := a.run(ctx, "merge", "--no-ff", "--no-edit", request.Source)
	return err
}

func (a Adapter) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.RepoDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			return "", fmt.Errorf("git %v: %w", args, err)
		}
		return "", fmt.Errorf("git %v: %w: %s", args, err, detail)
	}
	return stdout.String(), nil
}
