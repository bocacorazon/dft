package flow

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestRunnerExecutesAgentStepAndWritesAuditArtifacts(t *testing.T) {
	root := t.TempDir()
	runner := Runner{
		Agent:        agentstub.Adapter{},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	result, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:        "intake",
			Type:      StepAgent,
			AgentName: "dft-intake.agent.md",
			Prompt:    "Normalize demand",
			Demand:    "Build intake loop",
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("step result count = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].Status != StepSucceeded {
		t.Fatalf("step status = %q, want succeeded", result.Steps[0].Status)
	}

	stepDir := filepath.Join(root, ".dft", "runs", "run-123", "steps", "intake")
	for _, name := range []string{"prompt.md", "stdout.txt", "parsed.json"} {
		path := filepath.Join(stepDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", name, err)
		}
		assertJSONWhenParsedArtifact(t, path)
	}
}

func TestRunnerStopsOnFailedStep(t *testing.T) {
	runner := Runner{RunID: "run-123", ArtifactRoot: t.TempDir()}

	_, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:   "broken",
			Type: StepAgent,
		}},
	})

	if err == nil {
		t.Fatal("Execute returned nil error, want failure")
	}
}

func TestRunnerAttachesProjectContextAndWritesContextHashes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dft", "context"), 0o755); err != nil {
		t.Fatalf("create context dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dft", "context", "project.md"), []byte("Use mandatory TDD.\n"), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	agent := &capturingAgent{}
	runner := Runner{
		Agent:        agent,
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	_, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:        "intake",
			Type:      StepAgent,
			AgentName: "dft-intake.agent.md",
			Prompt:    "Normalize demand",
			Demand:    "Build intake loop",
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(agent.prompt, "Use mandatory TDD.") {
		t.Fatalf("agent prompt did not include project context: %q", agent.prompt)
	}
	stepDir := filepath.Join(root, ".dft", "runs", "run-123", "steps", "intake")
	content, err := os.ReadFile(filepath.Join(stepDir, "context-hashes.json"))
	if err != nil {
		t.Fatalf("read context hashes: %v", err)
	}
	var artifact struct {
		Context []contextHash `json:"context"`
	}
	if err := json.Unmarshal(content, &artifact); err != nil {
		t.Fatalf("context hashes invalid JSON: %v\n%s", err, content)
	}
	if len(artifact.Context) != 1 || artifact.Context[0].Path != ".dft/context/project.md" || len(artifact.Context[0].SHA256) != 64 {
		t.Fatalf("context hashes = %#v, want project context hash", artifact.Context)
	}
}

func TestRunnerCanDisableProjectContext(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dft", "context"), 0o755); err != nil {
		t.Fatalf("create context dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dft", "context", "project.md"), []byte("Use mandatory TDD.\n"), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	agent := &capturingAgent{}
	runner := Runner{Agent: agent, ArtifactRoot: root, RunID: "run-123"}

	_, err := runner.Execute(context.Background(), Definition{
		Steps: []Step{{
			ID:        "intake",
			Type:      StepAgent,
			AgentName: "dft-intake.agent.md",
			Prompt:    "Normalize demand",
			NoContext: true,
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(agent.prompt, "Use mandatory TDD.") {
		t.Fatalf("agent prompt included context despite no_context: %q", agent.prompt)
	}
}

func TestRunnerCapturesTextAgentOutputWithoutJSONParsing(t *testing.T) {
	root := t.TempDir()
	runner := Runner{
		Agent:        staticTextAgent{raw: "created files\n"},
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	_, err := runner.Execute(context.Background(), Definition{Steps: []Step{{
		ID:         "speckit",
		Type:       StepAgent,
		AgentName:  "speckit.specify.agent.md",
		OutputMode: AgentOutputText,
		Prompt:     "Create spec",
	}}})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	stepDir := filepath.Join(root, ".dft", "runs", "run-123", "steps", "speckit")
	stdout, err := os.ReadFile(filepath.Join(stepDir, "stdout.txt"))
	if err != nil {
		t.Fatalf("read stdout artifact: %v", err)
	}
	if string(stdout) != "created files\n" {
		t.Fatalf("stdout = %q, want text output", stdout)
	}
}

func TestRunnerRetriesJSONAgentOutputWhenFirstResponseIsOnlyProse(t *testing.T) {
	root := t.TempDir()
	agent := &sequenceAgent{responses: []string{
		"Inspecting the implementation before finalizing findings.\n",
		`{"approved":true,"findings":[]}`,
	}}
	runner := Runner{
		Agent:        agent,
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	_, err := runner.Execute(context.Background(), Definition{Steps: []Step{{
		ID:        "review",
		Type:      StepAgent,
		AgentName: "dft-code-review.agent.md",
		Prompt:    "Review implementation",
	}}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(agent.prompts) != 2 {
		t.Fatalf("agent invocations = %d, want 2", len(agent.prompts))
	}
	if !strings.Contains(agent.prompts[1], "Return ONLY a single valid JSON value") {
		t.Fatalf("retry prompt = %q, want JSON-only suffix", agent.prompts[1])
	}
	stepDir := filepath.Join(root, ".dft", "runs", "run-123", "steps", "review")
	firstAttempt, err := os.ReadFile(filepath.Join(stepDir, "stdout-attempt-1.txt"))
	if err != nil {
		t.Fatalf("read first-attempt artifact: %v", err)
	}
	if string(firstAttempt) != "Inspecting the implementation before finalizing findings.\n" {
		t.Fatalf("first attempt = %q", firstAttempt)
	}
	finalStdout, err := os.ReadFile(filepath.Join(stepDir, "stdout.txt"))
	if err != nil {
		t.Fatalf("read final stdout: %v", err)
	}
	if string(finalStdout) != "{\"approved\":true,\"findings\":[]}" {
		t.Fatalf("final stdout = %q", finalStdout)
	}
}

func TestRunnerExecutesGitHubPRFunctionsWithRemoteAudit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh fixture is POSIX-specific")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "fake-gh")
	if err := os.WriteFile(binary, []byte(`#!/usr/bin/env sh
case "$1 $2" in
  "pr create") printf '42\n' ;;
  "pr checks") printf 'checks passed\n' ;;
  "pr merge") printf 'merged\n' ;;
  *) printf 'unexpected %s\n' "$*" >&2; exit 2 ;;
esac
`), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	runner := Runner{ArtifactRoot: root, RunID: "run-123"}

	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{
		{ID: "create-pr", Type: StepFunction, Function: "gh_pr_create", Args: map[string]string{"head": "increment/run-123", "base": "main", "title": "Run 123", "dry_run": "false", "binary": binary}},
		{ID: "checks", Type: StepFunction, Function: "gh_pr_wait_checks", Args: map[string]string{"dry_run": "false", "binary": binary}},
		{ID: "merge", Type: StepFunction, Function: "gh_pr_merge", Args: map[string]string{"dry_run": "false", "binary": binary}},
	}})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Vars["pr_number"] != "42" {
		t.Fatalf("pr_number var = %q, want 42", result.Vars["pr_number"])
	}
	for _, stepID := range []string{"create-pr", "checks", "merge"} {
		if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "remote", stepID+".json")); err != nil {
			t.Fatalf("missing remote audit for %s: %v", stepID, err)
		}
	}
}

func TestRunnerExecutesAdditionalClosedSetFunctions(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	runner := Runner{ArtifactRoot: root, RunID: "run-123"}

	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{
		{ID: "message", Type: StepFunction, Function: "commit_message", Args: map[string]string{"title": "feat: test", "body": "body"}},
		{ID: "switch", Type: StepFunction, Function: "git_checkout_branch", Args: map[string]string{"branch": "feature/001-auth"}},
		{ID: "current", Type: StepFunction, Function: "git_branch_current"},
		{ID: "default", Type: StepFunction, Function: "git_default_branch"},
		{ID: "push", Type: StepFunction, Function: "git_push", Args: map[string]string{"remote": "origin", "branch": "main"}},
	}})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Vars["commit_message"] == "" || result.Vars["current_branch"] == "" || result.Vars["default_branch"] == "" {
		t.Fatalf("vars missing closed-set function outputs: %#v", result.Vars)
	}
	if result.Vars["current_branch"] != "feature/001-auth" {
		t.Fatalf("current_branch = %q, want feature/001-auth", result.Vars["current_branch"])
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "remote", "push.json")); err != nil {
		t.Fatalf("missing git_push remote audit: %v", err)
	}
}

func TestNormalizeBranchNameReturnsSwitchableBranch(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
	}{
		{input: "feature/001-auth", want: "feature/001-auth"},
		{input: "refs/heads/feature/001-auth", want: "feature/001-auth"},
		{input: "heads/feature/001-auth", want: "feature/001-auth"},
		{input: " increment/run-123\n", want: "increment/run-123"},
	} {
		if got := normalizeBranchName(tt.input); got != tt.want {
			t.Fatalf("normalizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRunnerWaitForHumanWritesInboxAndBlocks(t *testing.T) {
	root := t.TempDir()
	runner := Runner{ArtifactRoot: root, RunID: "run-123"}

	_, err := runner.Execute(context.Background(), Definition{Steps: []Step{{
		ID:       "approval",
		Type:     StepFunction,
		Function: "wait_for_human",
		Args:     map[string]string{"message": "approve"},
	}}})

	if err == nil {
		t.Fatalf("Execute returned nil error, want human gate block")
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "inbox", "run-123-approval.json")); err != nil {
		t.Fatalf("missing wait_for_human inbox item: %v", err)
	}
}

func TestRunnerSupportsPerStepVerificationRetryContinueEscalateWorkflowAndLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tool fixture uses POSIX sh")
	}
	root := t.TempDir()
	script := filepath.Join(root, "write-file.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env sh\nprintf '%s' \"$1\" > \"$2\"\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	workflowPath := filepath.Join(root, "workflow.yaml")
	content := `steps:
  - id: workflow-var
    type: function
    function: set_var
    args:
      name: workflow_value
      value: done
`
	if err := os.WriteFile(workflowPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
		Verifier:     verify.Checker{RootDir: root},
	}

	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{
		{
			ID:      "write",
			Type:    StepTool,
			Command: []string{script, "hello", filepath.Join(root, "result.txt")},
			Verify:  []domain.Check{{ID: "result", Kind: domain.CheckGrepMatches, Args: []string{"result.txt", "hello"}}},
		},
		{
			ID:      "continued",
			Type:    StepTool,
			Command: []string{"definitely-not-real"},
			OnError: "continue",
		},
		{
			ID:       "workflow",
			Type:     StepWorkflow,
			Workflow: workflowPath,
		},
		{
			ID:            "loop",
			Type:          StepLoop,
			MaxIterations: 2,
			ExitWhen:      map[string]string{"file_exists": "loop.txt"},
			Steps: []Step{{
				ID:      "loop-write",
				Type:    StepTool,
				Command: []string{script, "loop", filepath.Join(root, "loop.txt")},
			}},
		},
		{
			ID:      "escalated",
			Type:    StepTool,
			Command: []string{"definitely-not-real"},
			OnError: "escalate",
		},
	}})

	if err == nil {
		t.Fatalf("Execute returned nil error, want escalated failure")
	}
	if result.Vars["workflow_value"] != "done" {
		t.Fatalf("workflow var = %q, want done", result.Vars["workflow_value"])
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "inbox", "run-123-escalated.json")); err != nil {
		t.Fatalf("missing escalation inbox item: %v", err)
	}
	if len(result.Verification) == 0 || result.Verification[0].Status != domain.VerdictPass {
		t.Fatalf("verification = %#v, want passing per-step verification", result.Verification)
	}
}

func TestRunnerLoopExitWhenMatchesStepOutput(t *testing.T) {
	root := t.TempDir()
	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
	}

	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{{
		ID:            "loop",
		Type:          StepLoop,
		MaxIterations: 3,
		ExitWhen:      map[string]string{"step_output_equals": "signal.ready=yes"},
		Steps: []Step{{
			ID:       "signal",
			Type:     StepFunction,
			Function: "set_var",
			Args: map[string]string{
				"name":  "ready",
				"value": "yes",
			},
		}},
	}}})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := result.StepOutputs["loop"]["status"]; got != "succeeded" {
		t.Fatalf("loop status = %#v, want succeeded", got)
	}
	if got := result.StepOutputs["loop"]["iterations"]; got != 1 {
		t.Fatalf("loop iterations = %#v, want 1", got)
	}
}

func TestRunnerCommitsLocalMutatingStepsWhenEnabled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git fixture uses POSIX sh")
	}
	root := t.TempDir()
	initGitRepo(t, root)
	script := filepath.Join(root, "write-file.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env sh\nprintf '%s' \"$1\" > \"$2\"\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	runner := Runner{ArtifactRoot: root, RunID: "run-123", CommitLocalSteps: true}

	_, err := runner.Execute(context.Background(), Definition{Steps: []Step{{
		ID:      "write",
		Type:    StepTool,
		Command: []string{script, "tracked", filepath.Join(root, "tracked.txt")},
	}}})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	log := runGitForTest(t, root, "log", "-1", "--pretty=%B")
	if !strings.Contains(log, "Run-ID: run-123") || !strings.Contains(log, "Step-ID: write") {
		t.Fatalf("commit message missing dft trailers:\n%s", log)
	}
	status := runGitForTest(t, root, "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("worktree dirty after engine commit:\n%s", status)
	}
}

func TestRunnerFinalizesSquashMergeBackAndDeletesBranches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git fixture uses POSIX sh")
	}
	root := t.TempDir()
	remote := filepath.Join(t.TempDir(), "origin.git")
	initGitRepo(t, root)
	runGitForTest(t, root, "init", "--bare", remote)
	runGitForTest(t, root, "remote", "add", "origin", remote)
	runGitForTest(t, root, "push", "-u", "origin", "main")
	runGitForTest(t, root, "switch", "-c", "feature/001-auth")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("feature change\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitForTest(t, root, "commit", "-am", "feature change")
	runGitForTest(t, root, "push", "-u", "origin", "feature/001-auth")

	runner := Runner{ArtifactRoot: root, RunID: "run-123"}
	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{
		{ID: "rebase", Type: StepFunction, Function: "git_rebase_merge_back", Args: map[string]string{"source": "feature/001-auth", "target": "main"}},
		{ID: "finalize", Type: StepFunction, Function: "git_finalize_squash_merge_back", Args: map[string]string{"source": "feature/001-auth", "target": "main", "remote": "origin", "message": "chore: squash merge feature"}},
	}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	finalize := result.StepOutputs["finalize"]
	if got := finalize["trees_equal"]; got != true {
		t.Fatalf("trees_equal = %#v, want true", got)
	}
	if got := finalize["local_branch_deleted"]; got != true {
		t.Fatalf("local_branch_deleted = %#v, want true", got)
	}
	if got := finalize["remote_branch_deleted_or_missing"]; got != true {
		t.Fatalf("remote_branch_deleted_or_missing = %#v, want true", got)
	}
	if got := finalize["target_branch_released"]; got != true {
		t.Fatalf("target_branch_released = %#v, want true", got)
	}
	if got := strings.TrimSpace(runGitForTest(t, root, "log", "-1", "--pretty=%s")); got != "chore: squash merge feature" {
		t.Fatalf("last commit subject = %q", got)
	}
	if got := strings.TrimSpace(runGitForTest(t, root, "rev-parse", "--abbrev-ref", "HEAD")); got != "HEAD" {
		t.Fatalf("current branch after finalize = %q, want detached HEAD", got)
	}
	if output := strings.TrimSpace(runGitForTest(t, root, "branch", "--list", "feature/001-auth")); output != "" {
		t.Fatalf("feature branch still exists locally:\n%s", output)
	}
	if output := strings.TrimSpace(runGitForTest(t, root, "ls-remote", "--heads", "origin", "feature/001-auth")); output != "" {
		t.Fatalf("feature branch still exists remotely:\n%s", output)
	}
}

func TestRunnerFinalizesSquashMergeBackWithoutRemote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git fixture uses POSIX sh")
	}
	root := t.TempDir()
	initGitRepo(t, root)
	runGitForTest(t, root, "switch", "-c", "feature/001-auth")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("feature change\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitForTest(t, root, "commit", "-am", "feature change")

	runner := Runner{ArtifactRoot: root, RunID: "run-123"}
	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{
		{ID: "rebase", Type: StepFunction, Function: "git_rebase_merge_back", Args: map[string]string{"source": "feature/001-auth", "target": "main"}},
		{ID: "finalize", Type: StepFunction, Function: "git_finalize_squash_merge_back", Args: map[string]string{"source": "feature/001-auth", "target": "main", "remote": "origin", "message": "chore: squash merge feature"}},
	}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	finalize := result.StepOutputs["finalize"]
	if got := finalize["remote_branch_existed"]; got != false {
		t.Fatalf("remote_branch_existed = %#v, want false when no remote is configured", got)
	}
	if got := finalize["remote_branch_deleted_or_missing"]; got != true {
		t.Fatalf("remote_branch_deleted_or_missing = %#v, want true", got)
	}
}

func TestRunnerFinalizesSquashMergeBackReleasesTargetBranchForLaterWorktree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git fixture uses POSIX sh")
	}
	root := t.TempDir()
	initGitRepo(t, root)
	runGitForTest(t, root, "switch", "-c", "increment/run-123")
	runGitForTest(t, root, "switch", "main")

	firstWorktree := filepath.Join(t.TempDir(), "first")
	runGitForTest(t, root, "worktree", "add", "-b", "feature/first", firstWorktree, "increment/run-123")
	if err := os.WriteFile(filepath.Join(firstWorktree, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first worktree file: %v", err)
	}
	runGitForTest(t, firstWorktree, "add", "first.txt")
	runGitForTest(t, firstWorktree, "commit", "-m", "first change")
	if _, err := (Runner{ArtifactRoot: firstWorktree, RunID: "run-123"}).Execute(context.Background(), Definition{Steps: []Step{
		{ID: "rebase", Type: StepFunction, Function: "git_rebase_merge_back", Args: map[string]string{"source": "feature/first", "target": "increment/run-123"}},
		{ID: "finalize", Type: StepFunction, Function: "git_finalize_squash_merge_back", Args: map[string]string{"source": "feature/first", "target": "increment/run-123", "message": "first squash"}},
	}}); err != nil {
		t.Fatalf("first mergeback returned error: %v", err)
	}
	if got := strings.TrimSpace(runGitForTest(t, firstWorktree, "rev-parse", "--abbrev-ref", "HEAD")); got != "HEAD" {
		t.Fatalf("first worktree branch after finalize = %q, want detached HEAD", got)
	}

	secondWorktree := filepath.Join(t.TempDir(), "second")
	runGitForTest(t, root, "worktree", "add", "-b", "feature/second", secondWorktree, "increment/run-123")
	if err := os.WriteFile(filepath.Join(secondWorktree, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second worktree file: %v", err)
	}
	runGitForTest(t, secondWorktree, "add", "second.txt")
	runGitForTest(t, secondWorktree, "commit", "-m", "second change")
	if _, err := (Runner{ArtifactRoot: secondWorktree, RunID: "run-123"}).Execute(context.Background(), Definition{Steps: []Step{
		{ID: "rebase", Type: StepFunction, Function: "git_rebase_merge_back", Args: map[string]string{"source": "feature/second", "target": "increment/run-123"}},
		{ID: "finalize", Type: StepFunction, Function: "git_finalize_squash_merge_back", Args: map[string]string{"source": "feature/second", "target": "increment/run-123", "message": "second squash"}},
	}}); err != nil {
		t.Fatalf("second mergeback returned error: %v", err)
	}
}

func TestRunnerFinalizesSquashMergeBackWhenSourceBranchAlreadyDeleted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git fixture uses POSIX sh")
	}
	root := t.TempDir()
	initGitRepo(t, root)
	runGitForTest(t, root, "switch", "-c", "feature/001-auth")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("feature change\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitForTest(t, root, "commit", "-am", "feature change")
	runGitForTest(t, root, "switch", "main")
	runGitForTest(t, root, "merge", "--squash", "feature/001-auth")
	runGitForTest(t, root, "commit", "-m", "chore: squash merge feature")
	runGitForTest(t, root, "branch", "-D", "feature/001-auth")

	runner := Runner{ArtifactRoot: root, RunID: "run-123"}
	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{{
		ID:       "finalize",
		Type:     StepFunction,
		Function: "git_finalize_squash_merge_back",
		Args:     map[string]string{"source": "feature/001-auth", "target": "main", "remote": "origin", "message": "chore: squash merge feature"},
	}}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	finalize := result.StepOutputs["finalize"]
	if got := finalize["status"]; got != "already-finalized" {
		t.Fatalf("status = %#v, want already-finalized", got)
	}
	if got := finalize["local_branch_deleted"]; got != true {
		t.Fatalf("local_branch_deleted = %#v, want true", got)
	}
	if got := finalize["trees_equal"]; got != true {
		t.Fatalf("trees_equal = %#v, want true", got)
	}
}

func TestRunnerReportsRebaseConflictsForMergeBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git fixture uses POSIX sh")
	}
	root := t.TempDir()
	initGitRepo(t, root)
	runGitForTest(t, root, "switch", "-c", "feature/001-auth")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("feature change\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitForTest(t, root, "commit", "-am", "feature change")
	runGitForTest(t, root, "switch", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("main change\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitForTest(t, root, "commit", "-am", "main change")

	runner := Runner{ArtifactRoot: root, RunID: "run-123"}
	result, err := runner.Execute(context.Background(), Definition{Steps: []Step{{
		ID:       "mergeback",
		Type:     StepFunction,
		Function: "git_rebase_merge_back",
		Args:     map[string]string{"source": "feature/001-auth", "target": "main"},
	}}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := result.StepOutputs["mergeback"]["status"]; got != "conflict" {
		t.Fatalf("mergeback status = %#v, want conflict", got)
	}
	if got := result.StepOutputs["mergeback"]["phase"]; got != "rebase" {
		t.Fatalf("mergeback phase = %#v, want rebase", got)
	}
}

func assertJSONWhenParsedArtifact(t *testing.T, path string) {
	t.Helper()
	if filepath.Base(path) != "parsed.json" {
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read parsed artifact: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("parsed artifact is invalid JSON: %v\n%s", err, content)
	}
}

type capturingAgent struct {
	prompt string
}

func (a *capturingAgent) Invoke(_ context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	a.prompt = request.Prompt
	return ports.AgentResponse{Raw: `{"ok":true}`}, nil
}

type staticTextAgent struct {
	raw string
}

func (a staticTextAgent) Invoke(context.Context, ports.AgentRequest) (ports.AgentResponse, error) {
	return ports.AgentResponse{Raw: a.raw}, nil
}

type sequenceAgent struct {
	responses []string
	prompts   []string
}

func (a *sequenceAgent) Invoke(_ context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	a.prompts = append(a.prompts, request.Prompt)
	if len(a.responses) == 0 {
		return ports.AgentResponse{Raw: ""}, nil
	}
	response := a.responses[0]
	a.responses = a.responses[1:]
	return ports.AgentResponse{Raw: response}, nil
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "dft@example.test"},
		{"config", "user.name", "dft"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, output)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, output)
		}
	}
}

func runGitForTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
