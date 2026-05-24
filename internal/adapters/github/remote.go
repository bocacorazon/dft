package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Adapter records GitHub remote-only operations.
type Adapter struct {
	RootDir string
	DryRun  bool
	Binary  string
}

// PRRequest describes a pull request creation operation.
type PRRequest struct {
	RunID  string `json:"run_id"`
	StepID string `json:"step_id"`
	Head   string `json:"head"`
	Base   string `json:"base"`
	Title  string `json:"title"`
}

// PRRecord is the remote-only audit record for a PR operation.
type PRRecord struct {
	PRRequest
	RemoteOnly bool   `json:"remote_only"`
	Status     string `json:"status"`
	Number     int    `json:"number,omitempty"`
	Output     string `json:"output,omitempty"`
}

// CheckRequest describes waiting for checks on a pull request.
type CheckRequest struct {
	RunID  string `json:"run_id"`
	StepID string `json:"step_id"`
	Number int    `json:"number"`
}

// MergeRequest describes a pull request merge operation.
type MergeRequest struct {
	RunID  string `json:"run_id"`
	StepID string `json:"step_id"`
	Number int    `json:"number"`
	Method string `json:"method,omitempty"`
}

// BranchPRRequest describes looking up the PR number for a branch.
type BranchPRRequest struct {
	RunID  string `json:"run_id"`
	StepID string `json:"step_id"`
	Head   string `json:"head"`
}

// IssueRequest describes a GitHub issue creation operation.
type IssueRequest struct {
	RunID  string `json:"run_id"`
	StepID string `json:"step_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// IssueRecord is the remote-only audit record for an issue creation.
type IssueRecord struct {
	IssueRequest
	RemoteOnly bool   `json:"remote_only"`
	Status     string `json:"status"`
	Output     string `json:"output,omitempty"`
}

// RemoteRecord is the audit record for a remote-only GitHub operation.
type RemoteRecord struct {
	Operation  string `json:"operation"`
	RunID      string `json:"run_id"`
	StepID     string `json:"step_id"`
	RemoteOnly bool   `json:"remote_only"`
	Status     string `json:"status"`
	Number     int    `json:"number,omitempty"`
	Output     string `json:"output,omitempty"`
}

// CreatePR creates or dry-runs a pull request and writes a remote-only audit record.
func (a Adapter) CreatePR(ctx context.Context, request PRRequest) (PRRecord, error) {
	if request.RunID == "" || request.StepID == "" {
		return PRRecord{}, fmt.Errorf("run id and step id are required")
	}
	status := "dry_run"
	number := 0
	output := ""
	if !a.DryRun {
		args := []string{"pr", "create", "--head", request.Head, "--base", request.Base, "--title", request.Title, "--body", "Created by dft.", "--json", "number", "--jq", ".number"}
		out, err := a.run(ctx, args...)
		if err != nil {
			return PRRecord{}, err
		}
		output = out
		parsed, err := strconv.Atoi(strings.TrimSpace(out))
		if err != nil {
			return PRRecord{}, fmt.Errorf("parse PR number: %w", err)
		}
		number = parsed
		status = "created"
	}
	record := PRRecord{PRRequest: request, RemoteOnly: true, Status: status, Number: number, Output: output}
	if err := a.writeAudit(request.RunID, request.StepID, record); err != nil {
		return PRRecord{}, err
	}
	return record, nil
}

// PRNumberForBranch returns the pull request number for a branch.
func (a Adapter) PRNumberForBranch(ctx context.Context, request BranchPRRequest) (RemoteRecord, error) {
	if request.RunID == "" || request.StepID == "" {
		return RemoteRecord{}, fmt.Errorf("run id and step id are required")
	}
	record := RemoteRecord{Operation: "gh_pr_number_for_branch", RunID: request.RunID, StepID: request.StepID, RemoteOnly: true, Status: "dry_run"}
	if !a.DryRun {
		out, err := a.run(ctx, "pr", "list", "--head", request.Head, "--json", "number", "--jq", ".[0].number")
		if err != nil {
			return RemoteRecord{}, err
		}
		record.Output = out
		number, err := strconv.Atoi(strings.TrimSpace(out))
		if err != nil {
			return RemoteRecord{}, fmt.Errorf("parse PR number: %w", err)
		}
		record.Number = number
		record.Status = "found"
	}
	if err := a.writeAudit(request.RunID, request.StepID, record); err != nil {
		return RemoteRecord{}, err
	}
	return record, nil
}

// WaitChecks waits for PR checks through gh.
func (a Adapter) WaitChecks(ctx context.Context, request CheckRequest) (RemoteRecord, error) {
	if request.RunID == "" || request.StepID == "" {
		return RemoteRecord{}, fmt.Errorf("run id and step id are required")
	}
	record := RemoteRecord{Operation: "gh_pr_wait_checks", RunID: request.RunID, StepID: request.StepID, RemoteOnly: true, Status: "dry_run", Number: request.Number}
	if !a.DryRun {
		out, err := a.run(ctx, "pr", "checks", strconv.Itoa(request.Number), "--watch")
		if err != nil {
			return RemoteRecord{}, err
		}
		record.Output = out
		record.Status = "passed"
	}
	if err := a.writeAudit(request.RunID, request.StepID, record); err != nil {
		return RemoteRecord{}, err
	}
	return record, nil
}

// MergePR merges a pull request through gh.
func (a Adapter) MergePR(ctx context.Context, request MergeRequest) (RemoteRecord, error) {
	if request.RunID == "" || request.StepID == "" {
		return RemoteRecord{}, fmt.Errorf("run id and step id are required")
	}
	method := request.Method
	if method == "" {
		method = "squash"
	}
	record := RemoteRecord{Operation: "gh_pr_merge", RunID: request.RunID, StepID: request.StepID, RemoteOnly: true, Status: "dry_run", Number: request.Number}
	if !a.DryRun {
		out, err := a.run(ctx, "pr", "merge", strconv.Itoa(request.Number), "--"+method)
		if err != nil {
			return RemoteRecord{}, err
		}
		record.Output = out
		record.Status = "merged"
	}
	if err := a.writeAudit(request.RunID, request.StepID, record); err != nil {
		return RemoteRecord{}, err
	}
	return record, nil
}

// CreateIssue creates or dry-runs a GitHub issue and writes a remote-only audit record.
func (a Adapter) CreateIssue(ctx context.Context, request IssueRequest) (IssueRecord, error) {
	if request.RunID == "" || request.StepID == "" {
		return IssueRecord{}, fmt.Errorf("run id and step id are required")
	}
	record := IssueRecord{IssueRequest: request, RemoteOnly: true, Status: "dry_run"}
	if !a.DryRun {
		out, err := a.run(ctx, "issue", "create", "--title", request.Title, "--body", request.Body)
		if err != nil {
			return IssueRecord{}, err
		}
		record.Output = out
		record.Status = "created"
	}
	if err := a.writeAudit(request.RunID, request.StepID, record); err != nil {
		return IssueRecord{}, err
	}
	return record, nil
}

func (a Adapter) run(ctx context.Context, args ...string) (string, error) {
	binary := a.Binary
	if binary == "" {
		binary = "gh"
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = a.RootDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("gh %s failed: %w: %s", strings.Join(args, " "), err, detail)
	}
	return stdout.String(), nil
}

func (a Adapter) writeAudit(runID string, stepID string, record any) error {
	path := filepath.Join(a.RootDir, ".dft", "runs", runID, "remote", stepID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create remote audit directory: %w", err)
	}
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode remote audit record: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write remote audit record: %w", err)
	}
	return nil
}
