package ports

import "context"

// CreateBranchRequest describes a branch created from an explicit base ref.
type CreateBranchRequest struct {
	Name string
	Base string
}

// CreateWorktreeRequest describes a worktree created for a branch from a base ref.
type CreateWorktreeRequest struct {
	Path   string
	Branch string
	Base   string
}

// MergeRequest describes a source branch merged into a target branch.
type MergeRequest struct {
	Source string
	Target string
}

// GitPort captures git operations required by the dft branch topology.
type GitPort interface {
	DefaultBranch(context.Context) (string, error)
	CreateBranch(context.Context, CreateBranchRequest) error
	CreateWorktree(context.Context, CreateWorktreeRequest) error
	Merge(context.Context, MergeRequest) error
}
