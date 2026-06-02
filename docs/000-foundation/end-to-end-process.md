# Dark Factory Toolkit — End-to-End Process

This document describes the **full dft process** as an operator-facing flow.
It primarily focuses on **what each step consumes and what it produces**. In
the macro-process section, it also names the command/agent that performs each
step and whether that step is automatic or operator-driven.

It covers the core full-process path equivalent to:

- repository prepared with dft assets
- a demand is submitted for execution
- design, spec execution, evaluation, review, and merge are run

It does **not** describe intake-only submissions or dogfood-only add-on
artifacts.

## Primary process inputs

The full process starts with these inputs:

| Input | Description |
| --- | --- |
| Repository | The target Git repository where work will be planned, implemented, and merged. |
| dft assets | Provisioned flows, agents, context, and templates in the repository. |
| Demand | The requested outcome, either as submitted text or a demand-package equivalent. |
| Run ID | The identifier for one full execution attempt. |
| Default branch | The repository branch that represents the release baseline. |
| Runtime policy | Operator choices such as hold-increment vs final merge, retry limits, and adapter selection. |

## Preparation steps

These steps usually happen before the full process starts.

| Step | Inputs | Outputs |
| --- | --- | --- |
| `dft init` | Repository | Initial dft scaffold in the repository, including `.dft/`, required agent files, flows, and context files. |
| `dft sync` | Repository with existing dft assets | Updated in-repo dft assets aligned with the current toolkit version. |
| `dft submit --full` | Demand, repository, runtime policy | A new run under `.dft/runs/<run-id>/` and the start of the full orchestration process. |

## Macro process

This is the top-level process for one submitted demand.

| Step | Inputs | Outputs | Commands / agents used | Invocation |
| --- | --- | --- | --- | --- |
| 1. Intent / demand-package creation | Raw demand, operator context | `intent/demand-package.json` containing the normalized demand, acceptance criteria, and run identity. | Kickoff: `dft submit --full`; authoring: `dft-intake.agent.md` via `internal/intake.Service.CreateDemandPackage` | Human starts the run; the agent call is automatic after submission. |
| 2. Increment setup | Demand-package, repository default branch | A new increment branch for the run. | `WorktreeManager.BeginIncrement` (git branch creation from the default branch) | Automatic. |
| 3. WBS authoring | Demand-package | `design/wbs.json` describing the specs that make up the increment. | `dft-wbs-builder.agent.md` via `SpecPlanner.buildWBS` | Automatic. |
| 4. Lane assignment | Demand-package, WBS | `design/lane-assignments.json` assigning one lane per spec. | `dft-lane-selector.agent.md` via `SpecPlanner.selectLanes` | Automatic. |
| 5. Eval-surface authoring | Demand-package, WBS | `design/eval-surfaces.json` declaring the observable surfaces that eval will use. | `dft-eval-surface-author.agent.md` via `eval.SurfaceContractAuthor.Author` | Automatic. |
| 6. Spec execution loop | WBS, lane assignments, increment branch | One completed lane run per spec, merged back into the increment branch on success. | Current engine path: `.dft/flows/spec-lane.yaml` / `LoadSpecKitLane`, which runs `speckit.specify`, `speckit.plan`, `speckit.tasks`, `speckit.analyze`, `speckit.implement`, `dft-code-review.agent.md`, `gh_issues_from_findings`, `git_commit_all`, `git_rebase_merge_back`, and `dft-mergeback.agent.md` | Automatic in the full macro run; spec/plan gates inside the lane are auto-approved here. |
| 7. Eval readiness | Increment branch, eval surfaces, available artifacts | `eval/eval-ready.json` declaring whether the increment is ready for evaluation. | `eval.ReadinessGate.Check` via `eval.Orchestrator.Run` | Automatic. |
| 8. Eval-plan authoring | Demand-package, WBS, eval surfaces, readiness state | `eval/eval-plan.json` when readiness passes. | `dft-eval-plan-author.agent.md` via `eval.ArtifactOnlyPlanAuthor.Author` | Automatic. |
| 9. Eval execution | Eval plan, ready surfaces/artifacts | `eval/evaluation.json` with verdict, findings, and evidence references. | `eval.Executor.Execute` plus verifier-backed deterministic checks | Automatic. |
| 10. Fix planning on eval failure | Demand-package, eval findings | `fix-plan/wbs-amendment.json` or equivalent remediation plan describing new specs to add. | `dft-fix-planner.agent.md` via `review.FixPlanner.Plan`; remediation specs then re-enter the spec execution loop | Automatic when eval fails. |
| 11. Final review | Increment branch diff, evaluation pass state | `review/final-review.json` with approve/block decision and findings. | `dft-review.agent.md` via `review.FinalReviewer.Review` | Automatic. |
| 12. Fix planning on review failure | Demand-package, review findings | A remediation WBS amendment that adds new specs for review findings. | `dft-fix-planner.agent.md` via `review.FixPlanner.Plan`; remediation specs then re-enter spec execution and eval/review reruns | Automatic when final review blocks. |
| 13. Final merge | Approved increment, passing evaluation, default branch | Increment branch merged to the default branch, unless increment-hold policy is active. | `WorktreeManager.CompleteIncrement` (git merge of increment into the default branch) | Automatic unless increment-hold policy is active; if held, a human merges later. |
| 14. Run summary | Full run state | `macro-result.json` summarizing increment, design outputs, eval outputs, review outcome, and merge status. | `writeMacroResult` in `internal/orchestration/macro.go` | Automatic. |

Current implementation note: the lane-assignment artifact is authored and
persisted, but the macro runner currently executes the provisioned Speckit spec
lane for every spec. In other words, lane selection is recorded at design time,
but it does not yet switch the runtime to a different lane implementation.

## Spec execution loop

The following sequence runs once for each spec in the WBS.

### Spec-loop inputs

| Input | Description |
| --- | --- |
| Spec | The current unit of work from the WBS. |
| Feature directory | The spec workspace path for the current spec. |
| Spec branch | The branch/worktree used for the current spec. |
| Increment branch | The branch that accumulates successful specs for the run. |

### Spec-loop steps

| Step | Inputs | Outputs |
| --- | --- | --- |
| 1. Spec worktree creation | Current spec, increment branch | A spec worktree and spec branch for isolated execution. |
| 2. `specify` | Spec description, feature directory | `spec.md` and `checklists/requirements.md` for the spec. |
| 3. Capture feature directory | `specify` output | The resolved feature directory used by later steps. |
| 4. Ensure spec branch context | Spec branch, feature directory | Working branch aligned for spec execution. |
| 5. Build plan input | Spec ID, feature directory | The plan-stage prompt/input package. |
| 6. Build analyze input | Spec ID, feature directory, known artifact paths | The analyze-stage prompt/input package. |
| 7. Capture workflow branch | Current branch state | The branch identity used later by mergeback. |
| 8. Review spec gate | Generated spec artifacts | Human approval to proceed from spec to plan. |
| 9. `plan` | Spec ID, feature directory, spec artifacts | `plan.md`, `research.md`, and related plan artifacts. |
| 10. Review plan gate | Generated plan artifacts | Human approval to proceed from plan to tasks. |
| 11. Build tasks input | Spec ID, feature directory, spec + plan artifacts | The tasks-stage prompt/input package. |
| 12. `tasks` | Spec ID, feature directory, spec + plan artifacts | `tasks.md` for the spec. |
| 13. `analyze` | Spec ID, feature directory, `spec.md`, `plan.md`, `tasks.md` | Structured analysis output identifying blocking and non-blocking findings. |
| 14. Analyze gate | Parsed analyze output | Pass/continue signal based on blocking findings. |
| 15. `tasks` remediation (conditional) | Captured analyze output | Updated `tasks.md` after one remediation pass when blocking findings exist. |
| 16. Build implement input | Spec ID, feature directory, `tasks.md` | The implement-stage prompt/input package. |
| 17. `implement` / `code-review` loop | `tasks.md`, repo root, prior review findings if any | Updated code and tasks, plus review output; loop exits when there are no critical review findings or the loop limit is reached. |
| 18. Issue handoff | Review findings | Follow-up issues for remaining non-blocking findings when issue creation is appropriate. |
| 19. Commit before mergeback | Completed spec worktree | Final spec commit before integration. |
| 20. Capture mergeback branch | Current branch state | The exact source branch for mergeback. |
| 21. Mergeback attempt | Source/spec branch, increment branch | Rebased source branch ready for squash merge, or conflict state. |
| 22. Mergeback resolution (conditional) | Rebase-conflict state | Resolved rebase state ready for finalization. |
| 23. Mergeback finalization | Rebasing complete, source branch, increment branch | Squash merge committed into the increment branch; source branch deleted locally and remotely when applicable. |
| 24. Mergeback verification | Mergeback-finalize output, repository git state | Verified mergeback postconditions, including clean state and tree equality. |

## Step contracts inside the implement/review loop

This loop is bounded and repeatable.

| Step | Inputs | Outputs |
| --- | --- | --- |
| `implement` | Current `tasks.md`, spec context, repository root | Code and artifact changes for the spec, plus task-progress metadata. |
| `code-review` | Current implementation state in the spec workspace | Structured review findings with severity levels. |
| `review-clean` | Parsed code-review output | Pass/fail signal on whether critical findings remain. |
| remediation input refresh | Review findings | A narrowed implement input focused on unresolved review findings. |

## Evaluation phase

The evaluation phase is artifact-oriented and runs after spec work is merged
into the increment branch.

| Step | Inputs | Outputs |
| --- | --- | --- |
| 1. Surface-to-artifact binding | Eval surface contract, available artifacts/endpoints | Bound readiness targets for each declared surface. |
| 2. Readiness checks | Bound surfaces and declared readiness probes | `eval/eval-ready.json` with `pass` or `blocked` status and findings. |
| 3. Eval-plan authoring | Demand-package, WBS, eval surfaces, readiness metadata | `eval/eval-plan.json` describing the eval scenarios/checks to run. |
| 4. Eval execution | Eval plan, ready surfaces/artifacts | `eval/evaluation.json` with verdict, findings, coverage, and evidence references. |
| 5. Evidence capture | Eval execution outputs | `eval/evidence/` containing captured artifacts/logs from evaluation. |

## Failure and remediation paths

These paths are conditional.

| Trigger | Inputs | Outputs |
| --- | --- | --- |
| Analyze blocking findings | Analyze output | One `tasks` remediation pass for the current spec. |
| Critical review findings | Review findings | Additional implement/review loop iterations, up to the configured limit. |
| Eval blocked or failed | Evaluation findings | A WBS amendment adding remediation specs before re-evaluation. |
| Final review blocked | Review findings | A WBS amendment adding remediation specs before a new review pass. |
| Retry budget exhausted | Unresolved findings after bounded retries | An escalated item in inbox/blocked-run surfaces for operator attention. |

## Final states

The process ends in one of these states.

| Final state | Meaning | Typical outputs |
| --- | --- | --- |
| Passed and merged | Evaluation passed, final review approved, increment merged to default branch | `macro-result.json`, `review/final-review.json`, merged default branch. |
| Passed and held | Evaluation passed, final review approved, increment intentionally not merged yet | `macro-result.json` with increment-held state, increment branch preserved. |
| Blocked by eval | Readiness or eval failed and remediation is required | `eval/eval-ready.json` and/or `eval/evaluation.json`, plus `fix-plan/wbs-amendment.json`. |
| Blocked by final review | Evaluation passed but final review did not approve | `review/final-review.json`, plus a remediation WBS amendment. |
| Escalated | Automatic retries/remediations were exhausted | Escalation/inbox artifacts for operator follow-up. |

## Primary artifacts produced by the whole process

| Phase | Primary outputs |
| --- | --- |
| Intent | `intent/demand-package.json` |
| Design | `design/wbs.json`, `design/lane-assignments.json`, `design/eval-surfaces.json` |
| Per-spec execution | `spec.md`, `checklists/requirements.md`, `plan.md`, `research.md`, `tasks.md`, step-level run artifacts |
| Eval | `eval/eval-ready.json`, `eval/eval-plan.json`, `eval/evaluation.json`, `eval/evidence/` |
| Review | `review/final-review.json` |
| Remediation | `fix-plan/wbs-amendment.json` when applicable |
| Run summary | `macro-result.json` |

## Reading this process as a contract

The key idea is:

- each step accepts a clearly defined set of upstream artifacts or decisions
- each step produces a clearly defined downstream artifact, decision, or branch state
- later steps consume those outputs instead of reconstructing intent from
  hidden implementation context

That is the operational contract for the full dft process.
