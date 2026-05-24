package orchestration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
)

func TestBuildSpecKitLanePassesBranchAndFeatureDirectoryEnvironment(t *testing.T) {
	worktreePath := t.TempDir()
	if err := os.Mkdir(filepath.Join(worktreePath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	definition := BuildSpecKitLane(domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
		AcceptanceCriteria: []string{
			"specify writes spec.md and checklists/requirements.md",
		},
	}, SpecWorktree{
		Branch:          "spec/run-123/001-real-submit",
		IncrementBranch: "increment/run-123",
		WorktreePath:    worktreePath,
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": "feature/001-real-submit",
		},
	})

	if len(definition.Steps) < 10 {
		t.Fatalf("step count = %d, want expanded enhanced lane", len(definition.Steps))
	}
	specify := topLevelStep(t, definition, "specify")
	if got := specify.CommandInput; !strings.Contains(got, "Feature directory: specs/001-real-submit") || !strings.Contains(got, "Feature description:\nEnable real submit") {
		t.Fatalf("specify command input = %q, want feature directory and description", got)
	}
	if got := specify.CommandInput; strings.Contains(got, "go.mod") == false || strings.Contains(got, "do not create implementation source files") == false {
		t.Fatalf("specify command input = %q, want explicit specify-only guardrails", got)
	}
	if got := topLevelStep(t, definition, "plan").CommandInput; got != "{{ vars.plan_input }}" {
		t.Fatalf("step plan command input = %q, want explicit plan input", got)
	}
	if got := topLevelStep(t, definition, "analyze").CommandInput; got != "{{ vars.analyze_input }}" {
		t.Fatalf("step analyze command input = %q, want explicit analyze input", got)
	}
	for _, id := range []string{"specify", "plan"} {
		step := topLevelStep(t, definition, id)
		if got := step.Model; got != cheapWorkflowModel {
			t.Fatalf("step %s Model = %q, want %q", id, got, cheapWorkflowModel)
		}
		if got := step.Env["GIT_BRANCH_NAME"]; got != "feature/001-real-submit" {
			t.Fatalf("step %s GIT_BRANCH_NAME = %q", id, got)
		}
		wantFeatureDir := "specs/001-real-submit"
		if id != "specify" {
			wantFeatureDir = "{{ vars.feature_directory }}"
		}
		if got := step.Env["SPECIFY_FEATURE_DIRECTORY"]; got != wantFeatureDir {
			t.Fatalf("step %s SPECIFY_FEATURE_DIRECTORY = %q", id, got)
		}
		if got := step.Cwd; got != worktreePath {
			t.Fatalf("step %s Cwd = %q", id, got)
		}
		if !step.NoContext {
			t.Fatalf("step %s NoContext = false, want true", id)
		}
	}
	if got := topLevelStep(t, definition, "mergeback-attempt").Args["target"]; got != "increment/run-123" {
		t.Fatalf("mergeback target = %q, want increment branch", got)
	}
	if got := topLevelStep(t, definition, "mergeback-attempt").Args["source"]; got != "{{ vars.current_branch }}" {
		t.Fatalf("mergeback source = %q, want recaptured current branch", got)
	}
	commitIndex := mustTopLevelStepIndex(t, definition, "commit-before-mergeback")
	captureIndex := mustTopLevelStepIndex(t, definition, "capture-mergeback-branch")
	mergebackIndex := mustTopLevelStepIndex(t, definition, "mergeback-attempt")
	if !(commitIndex < captureIndex && captureIndex < mergebackIndex) {
		t.Fatalf("mergeback branch recapture order commit=%d capture=%d mergeback=%d, want commit < capture < mergeback", commitIndex, captureIndex, mergebackIndex)
	}
}

func TestBuildSpecKitLaneWithoutGitWorktreeSkipsBranchSwitching(t *testing.T) {
	worktreePath := t.TempDir()
	definition := BuildSpecKitLane(domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
	}, SpecWorktree{
		IncrementBranch: "increment/run-123",
		WorktreePath:    worktreePath,
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": "feature/001-real-submit",
		},
	})

	ensureBranch := topLevelStep(t, definition, "ensure-speckit-branch")
	if got := ensureBranch.When; got != "false" {
		t.Fatalf("ensure-speckit-branch.when = %q, want false when no git worktree exists", got)
	}

	mergeback := topLevelStep(t, definition, "mergeback-attempt")
	if got := mergeback.When; got != "false" {
		t.Fatalf("mergeback-attempt.when = %q, want false when no git worktree exists", got)
	}
	if got := mergeback.Args["target"]; got != "" {
		t.Fatalf("mergeback-attempt target = %q, want empty when no git worktree exists", got)
	}
}

func TestBuildBaseSpecKitFlowDoesNotBindWorktreeContext(t *testing.T) {
	definition := BuildBaseSpecKitFlow(domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
		AcceptanceCriteria: []string{
			"specify writes spec.md and checklists/requirements.md",
		},
	})

	for _, id := range []string{"specify", "plan"} {
		step := topLevelStep(t, definition, id)
		if got := step.Cwd; got != "" {
			t.Fatalf("step %s Cwd = %q, want empty before binding", id, got)
		}
		if got := step.Env["GIT_BRANCH_NAME"]; got != "" {
			t.Fatalf("step %s GIT_BRANCH_NAME = %q, want empty before binding", id, got)
		}
		wantFeatureDir := "specs/001-real-submit"
		if id != "specify" {
			wantFeatureDir = "{{ vars.feature_directory }}"
		}
		if got := step.Env["SPECIFY_FEATURE_DIRECTORY"]; got != wantFeatureDir {
			t.Fatalf("step %s SPECIFY_FEATURE_DIRECTORY = %q", id, got)
		}
	}
}

func TestBuildSpecKitLaneReadsPromptFileForSpecify(t *testing.T) {
	worktreePath := t.TempDir()
	promptPath := filepath.Join(worktreePath, "prompts", "001-real-submit.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("mkdir prompt dir: %v", err)
	}
	const prompt = "# Build spec\nUse the authored prompt only."
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	definition := BuildSpecKitLane(domain.SpecRef{
		ID:                 "001-real-submit",
		Description:        filepath.Join("prompts", "001-real-submit.md"),
		AcceptanceCriteria: []string{"specify writes spec.md"},
	}, SpecWorktree{
		Branch:          "spec/run-123/001-real-submit",
		IncrementBranch: "increment/run-123",
		WorktreePath:    worktreePath,
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": "feature/001-real-submit",
		},
	})

	if got := definition.Steps[0].CommandInput; !strings.Contains(got, prompt) || !strings.Contains(got, "Feature directory: specs/001-real-submit") {
		t.Fatalf("specify command input = %q, want wrapped prompt file content %q", got, prompt)
	}
}

func TestBuildSpecKitLanePrefersPromptPathForSpecify(t *testing.T) {
	worktreePath := t.TempDir()
	promptPath := filepath.Join(worktreePath, "prompts", "001-real-submit.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("mkdir prompt dir: %v", err)
	}
	const prompt = "# Build spec\nUse the dedicated prompt path."
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	definition := BuildSpecKitLane(domain.SpecRef{
		ID:                 "001-real-submit",
		Description:        "fallback description",
		PromptPath:         filepath.Join("prompts", "001-real-submit.md"),
		AcceptanceCriteria: []string{"specify writes spec.md"},
	}, SpecWorktree{
		IncrementBranch: "increment/run-123",
		WorktreePath:    worktreePath,
	})

	if got := definition.Steps[0].CommandInput; !strings.Contains(got, prompt) || !strings.Contains(got, "Feature directory: specs/001-real-submit") {
		t.Fatalf("specify command input = %q, want wrapped prompt_path content %q", got, prompt)
	}
}

func TestBuildSpecKitLaneUsesGenericArtifactDrivenPrompts(t *testing.T) {
	definition := BuildBaseSpecKitFlow(domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
	})

	implementLoop := topLevelStep(t, definition, "implement-review-loop")
	if got := implementLoop.ExitWhen["check_passes"]; got != "review-no-critical-findings" {
		t.Fatalf("implement-review-loop exit_when = %q, want review-no-critical-findings", got)
	}
	if implementLoop.MaxIterations != 3 {
		t.Fatalf("implement-review-loop max_iterations = %d, want 3", implementLoop.MaxIterations)
	}
	reviewClean, ok := findStep(implementLoop.Steps, "review-clean")
	if !ok {
		t.Fatal("implement-review-loop missing review-clean verify step")
	}
	if len(reviewClean.Checks) != 1 || reviewClean.Checks[0].ID != "review-no-critical-findings" {
		t.Fatalf("review-clean checks = %#v, want review-no-critical-findings", reviewClean.Checks)
	}

	planInput := topLevelStep(t, definition, "set-plan-input").Args["value"]
	if !strings.Contains(planInput, "Use repository-relative paths only") &&
		!strings.Contains(planInput, "use repository-relative paths only") {
		t.Fatalf("plan input = %q, want repository-relative path guidance", planInput)
	}
	if !strings.Contains(planInput, "failing tests are planned before production code changes") {
		t.Fatalf("plan input = %q, want test-first sequencing guidance", planInput)
	}
	if !strings.Contains(planInput, "fail/escalate on a conflicting existing module path") {
		t.Fatalf("plan input = %q, want non-destructive go.mod guidance", planInput)
	}
	if !strings.Contains(planInput, "Do not hard-code paths, command names, package names, requirement IDs, or version values") {
		t.Fatalf("plan input = %q, want generic no-hardcoding guidance", planInput)
	}
	if !strings.Contains(planInput, "Do not invent undefined quality/functional requirement identifiers") {
		t.Fatalf("plan input = %q, want undefined-requirement prohibition", planInput)
	}
	if !strings.Contains(planInput, "Replace all template placeholders") &&
		!strings.Contains(planInput, "replace all template placeholders") {
		t.Fatalf("plan input = %q, want placeholder-tree prohibition", planInput)
	}

	tasksInput := topLevelStep(t, definition, "set-tasks-input").Args["value"]
	if !strings.Contains(tasksInput, "Use repository-relative paths only") &&
		!strings.Contains(tasksInput, "use repository-relative paths only") {
		t.Fatalf("tasks input = %q, want repository-relative path guidance", tasksInput)
	}
	if !strings.Contains(tasksInput, "every production behavior change is preceded by a failing test") {
		t.Fatalf("tasks input = %q, want test-first task ordering guidance", tasksInput)
	}
	if !strings.Contains(tasksInput, "creates go.mod only when missing") {
		t.Fatalf("tasks input = %q, want non-destructive go.mod guidance", tasksInput)
	}
	if !strings.Contains(tasksInput, "Prefer integration/CI checks against built artifacts") &&
		!strings.Contains(tasksInput, "prefer integration/CI checks against built artifacts") {
		t.Fatalf("tasks input = %q, want built-artifact guidance", tasksInput)
	}
	if !strings.Contains(tasksInput, "Use only requirement IDs, command names, paths, version literals, and edge cases that are defined by the current spec/plan") {
		t.Fatalf("tasks input = %q, want current-spec-only guidance", tasksInput)
	}
	if !strings.Contains(tasksInput, "mark every task complete only after the corresponding work has actually been done") {
		t.Fatalf("tasks input = %q, want task completion guidance", tasksInput)
	}
	if strings.Contains(tasksInput, "init-go-cli") || strings.Contains(tasksInput, "internal/cli/version.go") {
		t.Fatalf("tasks input = %q, want no feature-specific hardcoded paths", tasksInput)
	}

	implementInput := topLevelStep(t, definition, "set-implement-input").Args["value"]
	if !strings.Contains(implementInput, "update tasks.md so every completed task is marked [X]") {
		t.Fatalf("implement input = %q, want task checkbox completion guidance", implementInput)
	}
	tasksRemediation := topLevelStep(t, definition, "tasks-remediation")
	if got := tasksRemediation.CommandInput; got != "fix the analyze findings -> {{ steps.analyze.output.stdout }}" {
		t.Fatalf("tasks-remediation input = %q, want exact captured-output prompt", got)
	}
}

func TestBuildSpecKitLaneUsesDSLChecksForTasksAnalyzeAndImplementValidation(t *testing.T) {
	definition := BuildBaseSpecKitFlow(domain.SpecRef{
		ID:          "001-real-submit",
		Description: "Enable real submit",
	})

	tasks := topLevelStep(t, definition, "tasks")
	if len(tasks.Verify) < 3 {
		t.Fatalf("tasks verify count = %d, want structural validation checks", len(tasks.Verify))
	}
	if tasks.Verify[2].ID != "tasks-contains-task-lines" {
		t.Fatalf("tasks verify[2].id = %q, want tasks-contains-task-lines", tasks.Verify[2].ID)
	}
	if tasks.Verify[2].Kind != domain.CheckGrepMatches {
		t.Fatalf("tasks verify[2].kind = %q, want %q", tasks.Verify[2].Kind, domain.CheckGrepMatches)
	}
	if got := tasks.Verify[2].Args[1]; got != "- [ ]" {
		t.Fatalf("tasks verify[2].args[1] = %q, want \"- [ ]\"", got)
	}

	implement := topLevelStep(t, definition, "implement")
	if len(implement.Verify) != 1 {
		t.Fatalf("implement verify count = %d, want 1 progress check", len(implement.Verify))
	}
	if implement.Verify[0].ID != "implement-task-progress" {
		t.Fatalf("implement verify[0].id = %q, want implement-task-progress", implement.Verify[0].ID)
	}
	if implement.Verify[0].Kind != domain.CheckStringEquals {
		t.Fatalf("implement verify[0].kind = %q, want %q", implement.Verify[0].Kind, domain.CheckStringEquals)
	}

	analyzeClean := topLevelStep(t, definition, "analyze-clean")
	if len(analyzeClean.Checks) != 1 || analyzeClean.Checks[0].ID != "no-blocking-analysis-findings" {
		t.Fatalf("analyze-clean checks = %#v, want blocking findings gate", analyzeClean.Checks)
	}
	tasksRemediation := topLevelStep(t, definition, "tasks-remediation")
	if len(tasksRemediation.Verify) != 2 {
		t.Fatalf("tasks-remediation verify count = %d, want 2 remediation checks", len(tasksRemediation.Verify))
	}
	resolveMergeback := topLevelStep(t, definition, "resolve-mergeback")
	if got := resolveMergeback.OutputMode; got != flow.AgentOutputText {
		t.Fatalf("resolve-mergeback output_mode = %q, want %q", got, flow.AgentOutputText)
	}
}

func TestLoadSpecKitLanePrefersProvisionedFlowFileWhenAvailable(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(root, ".dft", "flows", "spec-lane.yaml")
	if err := os.MkdirAll(filepath.Dir(flowPath), 0o755); err != nil {
		t.Fatalf("mkdir flow dir: %v", err)
	}
	content := `steps:
  - id: review-spec
    type: gate
    message: "{{ inputs.specify_input }}"
`
	if err := os.WriteFile(flowPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write flow file: %v", err)
	}

	definition, err := LoadSpecKitLane(root, domain.SpecRef{
		ID:                 "001-real-submit",
		Description:        "Enable real submit",
		AcceptanceCriteria: []string{"specify writes spec.md"},
	}, SpecWorktree{})
	if err != nil {
		t.Fatalf("LoadSpecKitLane returned error: %v", err)
	}

	if len(definition.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(definition.Steps))
	}
	if got := definition.Steps[0].Message; !strings.Contains(got, "Feature description:\nEnable real submit") {
		t.Fatalf("gate message = %q, want external flow rendering", got)
	}
}

func topLevelStep(t *testing.T, definition flow.Definition, id string) flow.Step {
	t.Helper()
	if step, ok := findStep(definition.Steps, id); ok {
		return step
	}
	t.Fatalf("step %q not found", id)
	return flow.Step{}
}

func mustTopLevelStepIndex(t *testing.T, definition flow.Definition, id string) int {
	t.Helper()
	index, err := topLevelStepIndex(definition, id)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func findStep(steps []flow.Step, id string) (flow.Step, bool) {
	for _, step := range steps {
		if step.ID == id {
			return step, true
		}
		if nested, ok := findStep(step.Setup, id); ok {
			return nested, true
		}
		if nested, ok := findStep(step.Steps, id); ok {
			return nested, true
		}
	}
	return flow.Step{}, false
}
