package flow

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/adapters/github"
)

// functionHandler runs one `function` step subcommand. root is the resolved
// working directory (step.Cwd or the artifact root) and stepDir is the step's
// audit artifact directory.
type functionHandler func(r Runner, ctx context.Context, step Step, root, stepDir string, result *Result) error

// functionHandlers maps a function-step name to its handler. Adding a new
// function subcommand is a single registry entry plus its method.
var functionHandlers = map[string]functionHandler{
	"set_var":                        Runner.fnSetVar,
	"gh_pr_create":                   Runner.fnGHPRCreate,
	"gh_pr_number_for_branch":        Runner.fnGHPRNumberForBranch,
	"gh_pr_wait_checks":              Runner.fnGHPRWaitChecks,
	"gh_pr_merge":                    Runner.fnGHPRMerge,
	"wait_for_human":                 Runner.fnWaitForHuman,
	"commit_message":                 Runner.fnCommitMessage,
	"git_branch_current":             Runner.fnGitBranchCurrent,
	"git_checkout_branch":            Runner.fnGitCheckoutBranch,
	"git_default_branch":             Runner.fnGitDefaultBranch,
	"branch":                         Runner.fnBranch,
	"merge":                          Runner.fnMerge,
	"git_push":                       Runner.fnGitPush,
	"git_commit_all":                 Runner.fnGitCommitAll,
	"gh_issues_from_findings":        Runner.fnGHIssuesFromFindings,
	"git_rebase_merge_back":          Runner.fnGitRebaseMergeBack,
	"git_finalize_squash_merge_back": Runner.fnGitFinalizeSquashMergeBack,
}

func (r Runner) fnSetVar(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	name := step.Args["name"]
	value := step.Args["value"]
	if name == "" {
		return fmt.Errorf("set_var requires name")
	}
	result.Vars[name] = value
	output := map[string]any{name: value}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGHPRCreate(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	record, err := githubAdapter(root, step).CreatePR(ctx, github.PRRequest{
		RunID:  r.RunID,
		StepID: step.ID,
		Head:   step.Args["head"],
		Base:   step.Args["base"],
		Title:  step.Args["title"],
	})
	if err != nil {
		return err
	}
	if record.Number != 0 {
		result.Vars["pr_number"] = strconv.Itoa(record.Number)
	}
	result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
		"number":      record.Number,
		"status":      record.Status,
		"remote_only": record.RemoteOnly,
		"output":      record.Output,
	})
	return writeParsed(stepDir, record)
}

func (r Runner) fnGHPRNumberForBranch(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	record, err := githubAdapter(root, step).PRNumberForBranch(ctx, github.BranchPRRequest{
		RunID:  r.RunID,
		StepID: step.ID,
		Head:   step.Args["head"],
	})
	if err != nil {
		return err
	}
	if record.Number != 0 {
		result.Vars["pr_number"] = strconv.Itoa(record.Number)
	}
	result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
		"number":      record.Number,
		"status":      record.Status,
		"remote_only": record.RemoteOnly,
		"output":      record.Output,
	})
	return writeParsed(stepDir, record)
}

func (r Runner) fnGHPRWaitChecks(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	number, err := stepPRNumber(step, result)
	if err != nil {
		return err
	}
	record, err := githubAdapter(root, step).WaitChecks(ctx, github.CheckRequest{RunID: r.RunID, StepID: step.ID, Number: number})
	if err != nil {
		return err
	}
	result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
		"number":      record.Number,
		"status":      record.Status,
		"remote_only": record.RemoteOnly,
		"output":      record.Output,
	})
	return writeParsed(stepDir, record)
}

func (r Runner) fnGHPRMerge(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	number, err := stepPRNumber(step, result)
	if err != nil {
		return err
	}
	record, err := githubAdapter(root, step).MergePR(ctx, github.MergeRequest{RunID: r.RunID, StepID: step.ID, Number: number, Method: step.Args["method"]})
	if err != nil {
		return err
	}
	result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{
		"number":      record.Number,
		"status":      record.Status,
		"remote_only": record.RemoteOnly,
		"output":      record.Output,
	})
	return writeParsed(stepDir, record)
}

func (r Runner) fnWaitForHuman(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	record := map[string]string{
		"status":  "blocked",
		"message": step.Args["message"],
	}
	if err := writeInboxItem(r.ArtifactRoot, r.RunID, step.ID, record); err != nil {
		return err
	}
	result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{"status": "blocked", "message": step.Args["message"]})
	if err := writeParsed(stepDir, record); err != nil {
		return err
	}
	return fmt.Errorf("wait_for_human blocked run")
}

func (r Runner) fnCommitMessage(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	message := strings.TrimSpace(step.Args["title"] + "\n\n" + step.Args["body"])
	if message == "" {
		return fmt.Errorf("commit_message requires title or body")
	}
	result.Vars["commit_message"] = message
	output := map[string]any{"commit_message": message}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGitBranchCurrent(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	if isGitRepo(ctx, root) {
		branch, err := currentGitBranch(ctx, root)
		if err != nil {
			return err
		}
		result.Vars["current_branch"] = branch
		output := map[string]any{"current_branch": branch}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	if branch := strings.TrimSpace(step.Env["GIT_BRANCH_NAME"]); branch != "" {
		result.Vars["current_branch"] = branch
		output := map[string]any{"current_branch": branch}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	output := map[string]any{"current_branch": ""}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGitCheckoutBranch(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	branch := strings.TrimSpace(step.Args["branch"])
	if branch == "" {
		branch = strings.TrimSpace(step.Env["GIT_BRANCH_NAME"])
	}
	if branch == "" {
		return fmt.Errorf("git_checkout_branch requires branch")
	}
	output := map[string]any{"branch": branch}
	if !isGitRepo(ctx, root) {
		output["status"] = "noop"
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	if _, err := runGit(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		if _, err := runGit(ctx, root, "switch", branch); err != nil {
			return err
		}
		output["status"] = "switched"
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	if _, err := runGit(ctx, root, "switch", "-c", branch); err != nil {
		return err
	}
	output["status"] = "created"
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGitDefaultBranch(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	branch, err := defaultBranch(ctx, root)
	if err != nil {
		return err
	}
	result.Vars["default_branch"] = branch
	output := map[string]any{"default_branch": branch}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnBranch(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	name := step.Args["name"]
	base := step.Args["base"]
	if name == "" {
		return fmt.Errorf("branch requires name")
	}
	args := []string{"switch", "-c", name}
	if base != "" {
		args = append(args, base)
	}
	if _, err := runGit(ctx, root, args...); err != nil {
		return err
	}
	output := map[string]any{"branch": name, "base": base}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnMerge(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	source := step.Args["source"]
	target := step.Args["target"]
	if source == "" || target == "" {
		return fmt.Errorf("merge requires source and target")
	}
	if _, err := runGit(ctx, root, "switch", target); err != nil {
		return err
	}
	if _, err := runGit(ctx, root, "merge", "--no-ff", "--no-edit", source); err != nil {
		return err
	}
	output := map[string]any{"source": source, "target": target, "status": "merged"}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGitPush(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	remote := step.Args["remote"]
	branch := step.Args["branch"]
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		return fmt.Errorf("git_push requires branch")
	}
	record := map[string]string{"remote": remote, "branch": branch, "remote_only": "true"}
	if step.Args["dry_run"] != "false" {
		if err := writeRemoteAudit(root, r.RunID, step.ID, record); err != nil {
			return err
		}
		result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{"remote": remote, "branch": branch, "remote_only": true})
		return writeParsed(stepDir, record)
	}
	if _, err := runGit(ctx, root, "push", remote, branch); err != nil {
		return err
	}
	record["status"] = "pushed"
	if err := writeRemoteAudit(root, r.RunID, step.ID, record); err != nil {
		return err
	}
	result.StepOutputs[step.ID] = cloneAnyMap(map[string]any{"remote": remote, "branch": branch, "remote_only": true, "status": "pushed"})
	return writeParsed(stepDir, record)
}

func (r Runner) fnGitCommitAll(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	message := strings.TrimSpace(step.Args["message"])
	if message == "" {
		message = strings.TrimSpace(step.Args["title"] + "\n\n" + step.Args["body"])
	}
	if message == "" {
		return fmt.Errorf("git_commit_all requires message, title, or body")
	}
	status, err := runGit(ctx, root, "status", "--porcelain")
	if err != nil {
		return err
	}
	output := map[string]any{"message": message}
	if strings.TrimSpace(status) == "" {
		output["status"] = "noop"
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	if _, err := runGit(ctx, root, "add", "-A"); err != nil {
		return err
	}
	if _, err := runGit(ctx, root, "commit", "-m", message); err != nil {
		return err
	}
	commit, err := runGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	output["status"] = "committed"
	output["commit"] = strings.TrimSpace(commit)
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGHIssuesFromFindings(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	sourceStep := step.Args["step"]
	if sourceStep == "" {
		return fmt.Errorf("gh_issues_from_findings requires step")
	}
	stepOutput, ok := result.StepOutputs[sourceStep]
	if !ok {
		return fmt.Errorf("gh_issues_from_findings step %q has no output", sourceStep)
	}
	rawFindings, ok := stepOutput["findings"].([]any)
	if !ok || len(rawFindings) == 0 {
		output := map[string]any{"source_step": sourceStep, "created": 0, "issues": []any{}}
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	issues := make([]any, 0, len(rawFindings))
	for i, finding := range rawFindings {
		record, err := githubAdapter(root, step).CreateIssue(ctx, github.IssueRequest{
			RunID:  r.RunID,
			StepID: fmt.Sprintf("%s-%d", step.ID, i+1),
			Title:  formatFindingIssueTitle(step.Args["title_prefix"], finding),
			Body:   formatFindingIssueBody(sourceStep, finding),
		})
		if err != nil {
			return err
		}
		issues = append(issues, map[string]any{
			"title":       record.Title,
			"status":      record.Status,
			"remote_only": record.RemoteOnly,
			"output":      record.Output,
		})
	}
	output := map[string]any{"source_step": sourceStep, "created": len(issues), "issues": issues}
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGitRebaseMergeBack(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	source := step.Args["source"]
	target := step.Args["target"]
	if source == "" || target == "" {
		return fmt.Errorf("git_rebase_merge_back requires source and target")
	}
	output := map[string]any{"source": source, "target": target}
	if _, err := runGit(ctx, root, "switch", source); err != nil {
		return err
	}
	if _, err := runGit(ctx, root, "rebase", target); err != nil {
		conflicted, detectErr := hasUnmergedFiles(ctx, root)
		if detectErr != nil {
			return detectErr
		}
		if conflicted {
			output["status"] = "conflict"
			output["phase"] = "rebase"
			output["message"] = err.Error()
			result.StepOutputs[step.ID] = cloneAnyMap(output)
			return writeParsed(stepDir, output)
		}
		return err
	}
	output["status"] = "rebased"
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}

func (r Runner) fnGitFinalizeSquashMergeBack(ctx context.Context, step Step, root, stepDir string, result *Result) error {
	source := step.Args["source"]
	target := step.Args["target"]
	if source == "" || target == "" {
		return fmt.Errorf("git_finalize_squash_merge_back requires source and target")
	}
	remote := strings.TrimSpace(step.Args["remote"])
	if remote == "" {
		remote = "origin"
	}
	message := strings.TrimSpace(step.Args["message"])
	if message == "" {
		message = fmt.Sprintf("chore: squash merge %s into %s", source, target)
	}
	output := map[string]any{
		"source": source,
		"target": target,
		"remote": remote,
	}
	if conflicted, err := hasUnmergedFiles(ctx, root); err != nil {
		return err
	} else if conflicted {
		return fmt.Errorf("cannot finalize mergeback with unresolved conflicts")
	}
	sourceExists, err := localBranchExists(ctx, root, source)
	if err != nil {
		return err
	}
	if !sourceExists {
		targetTree, err := revParseTree(ctx, root, target)
		if err != nil {
			return err
		}
		output["status"] = "already-finalized"
		output["source_tree"] = targetTree
		output["target_tree"] = targetTree
		output["trees_equal"] = true
		output["local_branch_deleted"] = true
		remoteExists, err := remoteBranchExists(ctx, root, remote, source)
		if err != nil {
			return err
		}
		output["remote_branch_existed"] = remoteExists
		if err := releaseBranchIfCurrent(ctx, root, target, output); err != nil {
			return err
		}
		if remoteExists {
			if _, err := runGit(ctx, root, "push", remote, "--delete", source); err != nil {
				return err
			}
		}
		output["remote_branch_deleted_or_missing"] = true
		result.StepOutputs[step.ID] = cloneAnyMap(output)
		return writeParsed(stepDir, output)
	}
	sourceTree, err := revParseTree(ctx, root, source)
	if err != nil {
		return err
	}
	if _, err := runGit(ctx, root, "switch", target); err != nil {
		return err
	}
	if _, err := runGit(ctx, root, "merge", "--squash", source); err != nil {
		return err
	}
	status, err := runGit(ctx, root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		output["status"] = "already-squashed"
	} else {
		if _, err := runGit(ctx, root, "commit", "-m", message); err != nil {
			return err
		}
		output["status"] = "merged"
		commit, err := runGit(ctx, root, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		output["commit"] = strings.TrimSpace(commit)
	}
	targetTree, err := revParseTree(ctx, root, "HEAD")
	if err != nil {
		return err
	}
	output["source_tree"] = sourceTree
	output["target_tree"] = targetTree
	output["trees_equal"] = sourceTree == targetTree
	if sourceTree != targetTree {
		return fmt.Errorf("squash merge result tree does not match source branch tree")
	}
	if _, err := runGit(ctx, root, "branch", "-D", source); err != nil {
		return err
	}
	output["local_branch_deleted"] = true
	if err := releaseBranchIfCurrent(ctx, root, target, output); err != nil {
		return err
	}
	remoteExists, err := remoteBranchExists(ctx, root, remote, source)
	if err != nil {
		return err
	}
	output["remote_branch_existed"] = remoteExists
	if remoteExists {
		if _, err := runGit(ctx, root, "push", remote, "--delete", source); err != nil {
			return err
		}
	}
	output["remote_branch_deleted_or_missing"] = true
	result.StepOutputs[step.ID] = cloneAnyMap(output)
	return writeParsed(stepDir, output)
}
