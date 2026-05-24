package orchestration

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
)

const (
	cheapWorkflowModel  = "gpt-5-mini"
	specKitLaneFlowPath = ".dft/flows/spec-lane.yaml"
)

func SpecKitLaneFlowYAML() string {
	return `schema_version: "1.0"
workflow:
  id: "dft-speckit-base"
  name: "dft Speckit Base Lane"
  version: "1.0.0"
  description: "Runs specify, plan, tasks, analysis, implement, review, issue handoff, and mergeback"
inputs:
  specify_input:
    type: string
    required: true
  spec_id:
    type: string
    required: true
  feature_directory:
    type: string
    required: true
  speckit_branch:
    type: string
    required: true
  increment_branch:
    type: string
    required: true
  increment_branch_enabled:
    type: string
    required: true
steps:
  - id: specify
    command: speckit.specify
    integration: copilot
    model: gpt-5-mini
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "{{ inputs.feature_directory }}"
    input:
      args: "{{ inputs.specify_input }}"
    verify:
      - id: spec-file
        kind: file_exists
        args:
          - "{{ steps.specify.output.artifacts.feature_directory }}/spec.md"
      - id: spec-differs-template
        kind: file_checksum_differs
        args:
          - "{{ steps.specify.output.artifacts.feature_directory }}/spec.md"
          - ".specify/templates/spec-template.md"
      - id: spec-requirements
        kind: file_exists
        args:
          - "{{ steps.specify.output.artifacts.feature_directory }}/checklists/requirements.md"
    on_error: retry(1)

  - id: capture-feature-directory
    type: function
    function: set_var
    args:
      name: feature_directory
      value: "{{ steps.specify.output.artifacts.feature_directory }}"

  - id: ensure-speckit-branch
    type: function
    function: git_checkout_branch
    when: "{{ inputs.increment_branch_enabled }}"
    args:
      branch: "{{ inputs.speckit_branch }}"

  - id: set-phase-input
    type: function
    function: set_var
    args:
      name: phase_input
      value: "Spec ID: {{ inputs.spec_id }}\nFeature directory: {{ vars.feature_directory }}\n"

  - id: set-plan-input
    type: function
    function: set_var
    args:
      name: plan_input
      value: "Spec ID: {{ inputs.spec_id }}\nFeature directory: {{ vars.feature_directory }}\nInstruction: Update the existing plan artifacts at these exact paths. Replace all template placeholders with concrete values, remove unused example or option blocks, and ensure every project tree or path example matches this feature's spec.md. Use repository-relative paths only. Do not hard-code paths, command names, package names, requirement IDs, or version values unless they appear in spec.md or are required by the current repository. Preserve test-first sequencing so failing tests are planned before production code changes. For Go work, treat go.mod as verify-or-create work: create it only when missing and fail/escalate on a conflicting existing module path instead of overwriting it. Prefer verification against built artifacts or explicitly declared observable surfaces. Do not invent undefined quality/functional requirement identifiers, audit-trail requirements, build-info tasks, extra commands, or edge cases unless they are explicitly present in spec.md. Do not mark constitution, test, or quality gates as PASS unless the required checks have actually passed.\n"

  - id: set-analyze-input
    type: function
    function: set_var
    args:
      name: analyze_input
      value: "Spec ID: {{ inputs.spec_id }}\nFeature directory: {{ vars.feature_directory }}\nSpec file: {{ vars.feature_directory }}/spec.md\nPlan file: {{ vars.feature_directory }}/plan.md\nTasks file: {{ vars.feature_directory }}/tasks.md\nInstruction: Analyze the current spec, plan, and tasks at these exact paths. This is a pre-implementation analysis pass: evaluate whether required implementation artifacts are planned and verified by the tasks, and do not treat artifacts that will be created during implementation, such as go.mod or source files, as blocking findings when the plan and tasks explicitly create and validate them before implementation.\n"

  - id: capture-workflow-branch
    type: function
    function: git_branch_current

  - id: review-spec
    type: gate
    message: "Review the generated spec before planning."

  - id: plan
    command: speckit.plan
    integration: copilot
    model: gpt-5-mini
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "{{ vars.feature_directory }}"
    input:
      args: "{{ vars.plan_input }}"
    verify:
      - id: plan-file
        kind: file_exists
        args:
          - "{{ steps.plan.output.artifacts.plan_file }}"
      - id: plan-differs-template
        kind: file_checksum_differs
        args:
          - "{{ steps.plan.output.artifacts.plan_file }}"
          - ".specify/templates/plan-template.md"
      - id: research-file
        kind: file_exists
        args:
          - "{{ steps.plan.output.artifacts.research_file }}"
    on_error: retry(1)

  - id: review-plan
    type: gate
    message: "Review the plan before generating tasks."

  - id: set-tasks-input
    type: function
    function: set_var
    args:
      name: tasks_input
      value: "Spec ID: {{ inputs.spec_id }}\nFeature directory: {{ vars.feature_directory }}\nInstruction: Generate tasks that cover every functional and quality requirement in spec.md and plan.md. Use only requirement IDs, command names, paths, version literals, and edge cases that are defined by the current spec/plan. Use repository-relative paths only. Include explicit verification tasks for required observable behavior, dependency constraints, CI/test gates, and measurable success criteria. Preserve test-first sequencing so every production behavior change is preceded by a failing test in the same scope. For Go work, add a prerequisite task that verifies go.mod when present, creates go.mod only when missing, and fails/escalates on a conflicting existing module path instead of overwriting it. Prefer integration/CI checks against built artifacts or declared observable surfaces instead of ambiguous go-run-only verification. Do not invent extra commands, packages, requirement identifiers, audit-trail requirements, build-info tasks, or unrelated edge cases. Annotate supporting tasks with traceability only to requirement IDs that actually exist in spec.md. Include final tasks to run the required test/build commands and to mark every task complete only after the corresponding work has actually been done.\n"

  - id: tasks
    command: speckit.tasks
    integration: copilot
    model: gpt-5-mini
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "{{ vars.feature_directory }}"
    input:
      args: "{{ vars.tasks_input }}"
    verify:
      - id: tasks-file
        kind: file_exists
        args:
          - "{{ steps.tasks.output.artifacts.tasks_file }}"
      - id: tasks-differs-template
        kind: file_checksum_differs
        args:
          - "{{ steps.tasks.output.artifacts.tasks_file }}"
          - ".specify/templates/tasks-template.md"
      - id: tasks-contains-task-lines
        kind: grep_matches
        args:
          - "{{ steps.tasks.output.artifacts.tasks_file }}"
          - "- [ ]"
    on_error: retry(1)

  - id: analyze
    command: speckit.analyze
    integration: copilot
    model: gpt-5-mini
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "{{ vars.feature_directory }}"
    input:
      args: "{{ vars.analyze_input }}"

  - id: analyze-clean
    type: verify
    checks:
      - id: no-blocking-analysis-findings
        kind: json_path_equals
        args:
          - "{{ inputs.artifact_root }}/.dft/runs/{{ inputs.run_id }}/steps/analyze/parsed.json"
          - "summary.blocking_findings"
          - "0"
    on_error: continue

  - id: tasks-remediation
    when: "{{ steps.analyze.output.summary.blocking_findings }}"
    command: speckit.tasks
    integration: copilot
    model: gpt-5-mini
    allow_tools: true
    no_context: true
    env:
      SPECIFY_FEATURE_DIRECTORY: "{{ vars.feature_directory }}"
    input:
      args: "fix the analyze findings -> {{ steps.analyze.output.stdout }}"
    verify:
      - id: remediation-tasks-file
        kind: file_exists
        args:
          - "{{ steps.tasks-remediation.output.artifacts.tasks_file }}"
      - id: remediation-tasks-differs-template
        kind: file_checksum_differs
        args:
          - "{{ steps.tasks-remediation.output.artifacts.tasks_file }}"
          - ".specify/templates/tasks-template.md"

  - id: set-implement-input
    type: function
    function: set_var
    args:
      name: implement_input
      value: "{{ vars.phase_input }}Instruction: Implement the tasks in {{ vars.feature_directory }}/tasks.md using the repository root as the project root. Complete tasks in dependency order, run the required verification commands, and update tasks.md so every completed task is marked [X]. Do not mark a task complete until its work and verification have actually succeeded. If a task cannot be completed, leave it unchecked and explain the blocker in the command output.\n"

  - id: implement-review-loop
    type: loop
    max_iterations: 3
    on_error: continue
    exit_when:
      check_passes: review-no-critical-findings
    steps:
      - id: implement
        command: speckit.implement
        integration: copilot
        model: gpt-5-mini
        allow_tools: true
        no_context: true
        env:
          SPECIFY_FEATURE_DIRECTORY: "{{ vars.feature_directory }}"
        input:
          args: "{{ vars.implement_input }}"
        verify:
          - id: implement-task-progress
            kind: string_equals
            args:
              - "{{ steps.implement.output.artifacts.task_progress }}"
              - "true"
      - id: code-review
        type: agent
        agent_name: dft-code-review.agent.md
        allow_tools: true
        no_context: true
        prompt: |
          Review the implementation in {{ vars.feature_directory }}.
          Return ONLY strict JSON using the dft review schema with severity per finding.
          Do not include prose, markdown, or code fences.
          Block only correctness, security, data-loss, auditability, or test coverage issues.
      - id: review-clean
        type: verify
        checks:
          - id: review-no-critical-findings
            kind: string_equals
            args:
              - "{{ steps.code-review.output.summary.critical_findings }}"
              - "0"
        on_error: continue
      - id: set-remediation-implement-input
        type: function
        function: set_var
        args:
          name: implement_input
          value: "Spec ID: {{ inputs.spec_id }}\nFeature directory: {{ vars.feature_directory }}\nInstruction: Remediate the remaining CRITICAL and HIGH review findings before finishing implementation.\n"

  - id: issues-from-review
    type: function
    function: gh_issues_from_findings
    args:
      step: code-review
      title_prefix: "Spec review follow-up"

  - id: commit-before-mergeback
    type: function
    function: git_commit_all
    when: "{{ inputs.increment_branch_enabled }}"
    args:
      message: "chore: finalize {{ vars.feature_directory }} before mergeback"

  - id: capture-mergeback-branch
    type: function
    function: git_branch_current
    when: "{{ inputs.increment_branch_enabled }}"

  - id: mergeback-attempt
    type: function
    function: git_rebase_merge_back
    when: "{{ inputs.increment_branch_enabled }}"
    args:
      source: "{{ vars.current_branch }}"
      target: "{{ inputs.increment_branch }}"

  - id: resolve-mergeback
    type: agent
    agent_name: dft-mergeback.agent.md
    output_mode: text
    allow_tools: true
    no_context: true
    when: "{{ inputs.increment_branch_enabled }}"
    prompt: |
      Ensure branch {{ vars.current_branch }} has been successfully rebased onto {{ inputs.increment_branch }}.
      If a rebase conflict is in progress, resolve it carefully and finish the rebase.
      Leave the repository clean and ready for the engine to perform the squash merge and branch deletion steps.
      If the rebase is already complete and the repository is clean, do nothing.

  - id: mergeback-finalize
    type: function
    function: git_finalize_squash_merge_back
    when: "{{ inputs.increment_branch_enabled }}"
    args:
      source: "{{ vars.current_branch }}"
      target: "{{ inputs.increment_branch }}"
      remote: "origin"
      message: "chore: squash merge {{ vars.feature_directory }} into {{ inputs.increment_branch }}"

  - id: verify-mergeback
    type: verify
    when: "{{ inputs.increment_branch_enabled }}"
    checks:
      - id: mergeback-no-conflicts
        kind: git_no_unmerged_files
      - id: mergeback-trees-equal
        kind: json_path_equals
        args:
          - "{{ inputs.artifact_root }}/.dft/runs/{{ inputs.run_id }}/steps/mergeback-finalize/parsed.json"
          - "trees_equal"
          - "true"
      - id: mergeback-local-branch-deleted
        kind: json_path_equals
        args:
          - "{{ inputs.artifact_root }}/.dft/runs/{{ inputs.run_id }}/steps/mergeback-finalize/parsed.json"
          - "local_branch_deleted"
          - "true"
      - id: mergeback-remote-branch-deleted
        kind: json_path_equals
        args:
          - "{{ inputs.artifact_root }}/.dft/runs/{{ inputs.run_id }}/steps/mergeback-finalize/parsed.json"
          - "remote_branch_deleted_or_missing"
          - "true"
`
}

// BuildBaseSpecKitFlow creates the Speckit phase flow without binding it to a worktree.
func BuildBaseSpecKitFlow(spec domain.SpecRef) flow.Definition {
	definition, err := loadSpecKitLane("", spec, SpecWorktree{})
	if err != nil {
		panic(err)
	}
	return definition
}

// BuildSpecKitLane binds the base Speckit flow to a specific worktree execution context.
func BuildSpecKitLane(spec domain.SpecRef, worktree SpecWorktree) flow.Definition {
	definition, err := loadSpecKitLane("", spec, worktree)
	if err != nil {
		panic(err)
	}
	return definition
}

func LoadSpecKitLane(root string, spec domain.SpecRef, worktree SpecWorktree) (flow.Definition, error) {
	return loadSpecKitLane(root, spec, worktree)
}

func loadSpecKitLane(root string, spec domain.SpecRef, worktree SpecWorktree) (flow.Definition, error) {
	template, err := loadSpecKitLaneTemplate(root)
	if err != nil {
		return flow.Definition{}, err
	}
	definition := flow.BindInputs(template, specKitFlowInputs(spec, worktree))
	definition = flow.BindDefinition(definition, flow.ExecutionContext{
		Cwd: worktree.WorktreePath,
		Env: worktree.SpecKitEnv,
	})
	if shouldValidateBuiltInSpecKitLane(root) {
		if err := validateSpecKitLaneDefinition(definition); err != nil {
			return flow.Definition{}, err
		}
	}
	return definition, nil
}

func shouldValidateBuiltInSpecKitLane(root string) bool {
	if root == "" {
		return true
	}
	_, err := os.Stat(filepath.Join(root, specKitLaneFlowPath))
	return os.IsNotExist(err)
}

func loadSpecKitLaneTemplate(root string) (flow.Definition, error) {
	if root != "" {
		path := filepath.Join(root, specKitLaneFlowPath)
		if _, err := os.Stat(path); err == nil {
			return flow.LoadDefinition(path)
		} else if !os.IsNotExist(err) {
			return flow.Definition{}, err
		}
	}
	return flow.ParseDefinition([]byte(SpecKitLaneFlowYAML()))
}

func specKitFlowInputs(spec domain.SpecRef, worktree SpecWorktree) map[string]any {
	featureDir := filepath.ToSlash(filepath.Join("specs", spec.ID))
	hasGitWorktree := worktreeHasGit(worktree.WorktreePath)
	incrementBranch := ""
	if worktree.IncrementBranch != "" && hasGitWorktree {
		incrementBranch = worktree.IncrementBranch
	}
	speckitBranch := ""
	if hasGitWorktree {
		speckitBranch = worktree.SpecKitEnv["GIT_BRANCH_NAME"]
	}
	return map[string]any{
		"feature_directory":        featureDir,
		"speckit_branch":           speckitBranch,
		"increment_branch":         incrementBranch,
		"increment_branch_enabled": strconv.FormatBool(incrementBranch != ""),
		"spec_id":                  spec.ID,
		"specify_input":            specKitSpecifyInput(spec, worktree.WorktreePath, featureDir),
	}
}

func specKitSpecifyInput(spec domain.SpecRef, worktreePath string, featureDir string) string {
	description := ""
	if content, ok := readPromptFile(spec.PromptPath, worktreePath); ok {
		description = content
	} else if content, ok := readPromptFile(spec.Description, worktreePath); ok {
		description = content
	} else {
		description = strings.TrimSpace(spec.Description)
	}
	return strings.TrimSpace("Feature directory: " + featureDir + `
Instruction: Run only the Speckit specify phase. Use this exact feature directory, create or update only the specification artifacts for this phase, and do not create implementation source files, go.mod, binaries, plan.md, research.md, or tasks.md. The required outputs are spec.md and checklists/requirements.md under the feature directory.

Feature description:
` + description)
}

func readPromptFile(description string, worktreePath string) (string, bool) {
	raw := strings.TrimSpace(description)
	if raw == "" {
		return "", false
	}
	candidates := []string{raw}
	if worktreePath != "" && !filepath.IsAbs(raw) {
		candidates = append(candidates, filepath.Join(worktreePath, raw))
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	}
	return "", false
}

func worktreeHasGit(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
