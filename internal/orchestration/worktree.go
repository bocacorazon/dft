package orchestration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// WorktreeManager owns the increment/spec branch topology for a run.
type WorktreeManager struct {
	Git          ports.GitPort
	WorktreeRoot string
}

// IncrementRequest identifies the increment branch envelope to create.
type IncrementRequest struct {
	RunID string
}

// Increment records branch metadata for one demand-package increment.
type Increment struct {
	RunID         string
	DefaultBranch string
	Branch        string
}

// SpecRequest identifies a spec branch/worktree to create under an increment.
type SpecRequest struct {
	RunID           string
	SpecID          string
	IncrementBranch string
}

// SpecWorktree records per-spec branch/worktree metadata and Spec Kit env.
type SpecWorktree struct {
	RunID           string
	SpecID          string
	Branch          string
	IncrementBranch string
	WorktreePath    string
	SpecKitEnv      map[string]string
}

// CompleteSpecRequest identifies a successful spec branch mergeback.
type CompleteSpecRequest struct {
	SpecBranch      string
	IncrementBranch string
}

// CompleteIncrementRequest identifies the final reviewed increment merge.
type CompleteIncrementRequest struct {
	IncrementBranch string
	DefaultBranch   string
	Evaluation      domain.VerificationResult
	Review          domain.ReviewDecision
}

// BeginIncrement creates an increment branch from the repository default branch.
func (m WorktreeManager) BeginIncrement(ctx context.Context, request IncrementRequest) (Increment, error) {
	if err := validateID("run id", request.RunID); err != nil {
		return Increment{}, err
	}
	if m.Git == nil {
		return Increment{}, fmt.Errorf("git port is required")
	}

	defaultBranch, err := m.Git.DefaultBranch(ctx)
	if err != nil {
		return Increment{}, fmt.Errorf("resolve default branch: %w", err)
	}
	if err := validateRef("default branch", defaultBranch); err != nil {
		return Increment{}, err
	}

	branch := "increment/" + request.RunID
	if err := m.Git.CreateBranch(ctx, ports.CreateBranchRequest{Name: branch, Base: defaultBranch}); err != nil {
		return Increment{}, fmt.Errorf("create increment branch %q from %q: %w", branch, defaultBranch, err)
	}

	return Increment{
		RunID:         request.RunID,
		DefaultBranch: defaultBranch,
		Branch:        branch,
	}, nil
}

// BeginSpec creates a per-spec branch and worktree from the increment branch.
func (m WorktreeManager) BeginSpec(ctx context.Context, request SpecRequest) (SpecWorktree, error) {
	if err := validateID("run id", request.RunID); err != nil {
		return SpecWorktree{}, err
	}
	if err := validateID("spec id", request.SpecID); err != nil {
		return SpecWorktree{}, err
	}
	if err := validateRef("increment branch", request.IncrementBranch); err != nil {
		return SpecWorktree{}, err
	}
	if m.Git == nil {
		return SpecWorktree{}, fmt.Errorf("git port is required")
	}

	root := m.WorktreeRoot
	if root == "" {
		root = filepath.Join(".dft", "worktrees")
	}
	branch := "spec/" + request.RunID + "/" + request.SpecID
	path := filepath.Join(root, request.RunID, request.SpecID)
	if err := m.Git.CreateWorktree(ctx, ports.CreateWorktreeRequest{
		Path:   path,
		Branch: branch,
		Base:   request.IncrementBranch,
	}); err != nil {
		return SpecWorktree{}, fmt.Errorf("create spec worktree %q from %q: %w", branch, request.IncrementBranch, err)
	}

	return SpecWorktree{
		RunID:           request.RunID,
		SpecID:          request.SpecID,
		Branch:          branch,
		IncrementBranch: request.IncrementBranch,
		WorktreePath:    path,
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": SpecKitFeatureBranchName(request.SpecID),
		},
	}, nil
}

// CompleteSpec merges a successful spec branch back into its increment branch.
func (m WorktreeManager) CompleteSpec(ctx context.Context, request CompleteSpecRequest) error {
	if err := validateRef("spec branch", request.SpecBranch); err != nil {
		return err
	}
	if err := validateRef("increment branch", request.IncrementBranch); err != nil {
		return err
	}
	if m.Git == nil {
		return fmt.Errorf("git port is required")
	}
	if err := m.Git.Merge(ctx, ports.MergeRequest{Source: request.SpecBranch, Target: request.IncrementBranch}); err != nil {
		return fmt.Errorf("merge spec branch %q into %q: %w", request.SpecBranch, request.IncrementBranch, err)
	}
	return nil
}

// CompleteIncrement merges an increment branch back to default after gates pass.
func (m WorktreeManager) CompleteIncrement(ctx context.Context, request CompleteIncrementRequest) error {
	if err := validateRef("increment branch", request.IncrementBranch); err != nil {
		return err
	}
	if err := validateRef("default branch", request.DefaultBranch); err != nil {
		return err
	}
	if request.Evaluation.Status != domain.VerdictPass {
		return fmt.Errorf("cannot merge increment: evaluation status is %q", request.Evaluation.Status)
	}
	if !request.Review.Approved {
		return fmt.Errorf("cannot merge increment: final review is not approved")
	}
	if m.Git == nil {
		return fmt.Errorf("git port is required")
	}
	if err := m.Git.Merge(ctx, ports.MergeRequest{Source: request.IncrementBranch, Target: request.DefaultBranch}); err != nil {
		return fmt.Errorf("merge increment branch %q into %q: %w", request.IncrementBranch, request.DefaultBranch, err)
	}
	return nil
}

func validateID(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.Contains(value, "..") || strings.ContainsAny(value, " \t\n\\") {
		return fmt.Errorf("%s %q contains unsupported characters", name, value)
	}
	return nil
}

func validateRef(name string, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.Contains(value, "..") || strings.ContainsAny(value, " \t\n\\") {
		return fmt.Errorf("%s %q contains unsupported characters", name, value)
	}
	return nil
}

// SpecKitFeatureBranchName returns the explicit inner branch name passed to Speckit hooks.
func SpecKitFeatureBranchName(specID string) string {
	return "feature/" + specID
}
