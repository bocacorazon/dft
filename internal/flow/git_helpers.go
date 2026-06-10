package flow

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/adapters/github"
)

func githubAdapter(root string, step Step) github.Adapter {
	return github.Adapter{
		RootDir: root,
		DryRun:  step.Args["dry_run"] != "false",
		Binary:  step.Args["binary"],
	}
}

func stepPRNumber(step Step, result *Result) (int, error) {
	value := step.Args["number"]
	if value == "" {
		value = result.Vars["pr_number"]
	}
	if value == "" {
		return 0, fmt.Errorf("%s requires PR number", step.Function)
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse PR number: %w", err)
	}
	return number, nil
}

func runGit(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func defaultBranch(ctx context.Context, root string) (string, error) {
	out, err := runGit(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(strings.TrimSpace(out), "origin/"), nil
	}
	out, err = runGit(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func revParseTree(ctx context.Context, root string, ref string) (string, error) {
	output, err := runGit(ctx, root, "rev-parse", ref+"^{tree}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func isGitRepo(ctx context.Context, root string) bool {
	_, err := runGit(ctx, root, "rev-parse", "--git-dir")
	return err == nil
}

func remoteBranchExists(ctx context.Context, root string, remote string, branch string) (bool, error) {
	if _, err := runGit(ctx, root, "remote", "get-url", remote); err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "no such remote") || strings.Contains(lower, "not a git repository") {
			return false, nil
		}
		return false, err
	}
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--exit-code", "--heads", remote, branch)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(output)) != "", nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 2 {
		return false, nil
	}
	return false, fmt.Errorf("git ls-remote --exit-code --heads %s %s failed: %w: %s", remote, branch, err, strings.TrimSpace(string(output)))
}

func localBranchExists(ctx context.Context, root string, branch string) (bool, error) {
	output, err := runGit(ctx, root, "branch", "--list", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func currentGitBranch(ctx context.Context, root string) (string, error) {
	output, err := runGit(ctx, root, "branch", "--show-current")
	if err == nil {
		return strings.TrimSpace(output), nil
	}
	output, err = runGit(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return normalizeBranchName(strings.TrimSpace(output)), nil
}

func normalizeBranchName(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	branch = strings.TrimPrefix(branch, "heads/")
	return branch
}

func releaseBranchIfCurrent(ctx context.Context, root string, branch string, output map[string]any) error {
	current, err := currentGitBranch(ctx, root)
	if err != nil {
		return err
	}
	if current != branch {
		output["target_branch_released"] = false
		return nil
	}
	if _, err := runGit(ctx, root, "switch", "--detach", "HEAD"); err != nil {
		return fmt.Errorf("release branch %q after mergeback: %w", branch, err)
	}
	output["target_branch_released"] = true
	return nil
}

func hasUnmergedFiles(ctx context.Context, root string) (bool, error) {
	output, err := runGit(ctx, root, "ls-files", "--unmerged")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func formatFindingIssueTitle(prefix string, finding any) string {
	details, ok := finding.(map[string]any)
	if !ok {
		if prefix == "" {
			prefix = "Follow-up"
		}
		return strings.TrimSpace(prefix + ": unresolved finding")
	}
	severity := normalizeSeverity(fmt.Sprint(details["severity"]))
	if severity == "" {
		severity = "HIGH"
	}
	message := strings.TrimSpace(fmt.Sprint(details["message"]))
	if message == "" {
		message = "unresolved finding"
	}
	if prefix == "" {
		prefix = "Follow-up"
	}
	return strings.TrimSpace(prefix + " [" + severity + "]: " + message)
}

func formatFindingIssueBody(sourceStep string, finding any) string {
	details, ok := finding.(map[string]any)
	if !ok {
		return "Created by dft from unresolved findings in step " + sourceStep + "."
	}
	var builder strings.Builder
	builder.WriteString("Created by dft from unresolved findings in step `")
	builder.WriteString(sourceStep)
	builder.WriteString("`.\n\n")
	if value := strings.TrimSpace(fmt.Sprint(details["finding_id"])); value != "" && value != "<nil>" {
		builder.WriteString("- Finding ID: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	if value := normalizeSeverity(fmt.Sprint(details["severity"])); value != "" {
		builder.WriteString("- Severity: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	if value := strings.TrimSpace(fmt.Sprint(details["category"])); value != "" && value != "<nil>" {
		builder.WriteString("- Category: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	if value := strings.TrimSpace(fmt.Sprint(details["location"])); value != "" && value != "<nil>" {
		builder.WriteString("- Location: `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}
	builder.WriteString("\n## Summary\n\n")
	builder.WriteString(strings.TrimSpace(fmt.Sprint(details["message"])))
	if value := strings.TrimSpace(fmt.Sprint(details["recommendation"])); value != "" && value != "<nil>" {
		builder.WriteString("\n\n## Recommendation\n\n")
		builder.WriteString(value)
	}
	return builder.String()
}
