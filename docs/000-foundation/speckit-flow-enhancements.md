
# Dark Factory Toolkit — Speckit Inner-Loop Plan

> Focused implementation plan for making the **single-spec Speckit lane**
> trustworthy before expanding macro orchestration around it.

## Scope

This document covers only the inner loop for one spec:

`specify -> plan -> tasks -> analyze -> implement -> code-review -> issue handoff -> mergeback`

Out of scope here:

- increment setup
- WBS creation and lane assignment
- eval / fix-planner
- final increment review
- final merge to the default branch

## Why this needed a reset

The original lane existed, but the operational foundation was weak in the
places that matter most:

- stage detection leaned on runner/journal state instead of real artifacts
- analyze remediation could loop in the wrong shape
- implement iterations could fail for partial progress instead of behaving like a bounded review loop
- mergeback recovery was not yet robust enough for real interrupted runs

The next phase of work should optimize for **correctness,
inspectability, and repeatability**, not for broader feature coverage.

## Core design direction

### 1. Use DSL step validation as the primary gate mechanism

The inner loop should not invent a parallel validation system.

Instead:

- use `verify:` blocks on command steps where the check belongs to the
  stage output
- use named `type: verify` steps where validation needs to be separated,
  reused, or referenced by later loop logic
- add new check kinds only when the current DSL checks cannot express
  the desired gate cleanly

The same validation definitions should be reused for:

- first-run stage gating
- resume decisions
- inspect/status summaries

### 2. Make artifact truth authoritative

The lane now recognizes state from the spec workspace itself:

- `specify` => `spec.md` exists and is not template output
- `plan` => `plan.md` and `research.md` exist and are not template output
- `tasks` => `tasks.md` exists and is not template output
- `implement` => every task in `tasks.md` is marked `[x]` or `[X]`

Stage selection, status, and recovery should be derived from those rules
plus persisted verification artifacts, not from the saved runner index.

### 3. Keep the lane journal as audit/debug data

Every stage attempt should still record:

- lane execution ID
- stage name
- attempt number
- status
- rendered verification checks
- verification results
- findings summary
- artifact locations
- recommended next action

But the journal is for **auditability and debugging**, not for deciding
which stage is complete.

### 4. Make loops explicit and inspectable

The lane has two important inner loops:

1. `tasks -> analyze -> remediation`
2. `implement -> code-review -> remediation`

For `tasks -> analyze -> remediation`:

- capture the full analyze output
- parse findings severity from the captured output
- if any finding is `HIGH` or `CRITICAL`, run exactly one remediation
  `speckit.tasks` pass with:
  `fix the analyze findings -> {captured-output}`
- continue forward after that remediation instead of re-entering an
  open-ended analyze loop

For `implement -> code-review -> remediation`:

- allow partial task completion inside an implement iteration
- treat stage completion separately from per-iteration progress
- run at most 3 implement/review attempts
- exit early when review returns no `CRITICAL` findings

### 5. Keep `resume` as an explicit operator command

Even with artifact-driven recovery, keeping `dft resume <run-id>` is still
useful as the explicit operator surface for restarting interrupted work.

The important design constraint is that `resume` must be a thin wrapper
over artifact-based stage selection rather than a special state machine of
its own.

### 6. Make mergeback a first-class stage

Mergeback should have:

- preconditions
- execution steps
- conflict state
- recovery path
- postconditions verified by git facts

Target behavior for this stage:

1. Rebase the source/spec branch onto the target/increment branch.
2. If rebase conflicts occur, invoke the mergeback LLM to resolve them
   and continue the rebase.
3. Switch to the target branch.
4. Squash-merge the rebased source branch into the target branch.
5. Create one engine-owned squash commit on the target branch.
6. Verify the target tree matches the source tree.
7. Delete the local spec branch.
8. Delete the remote spec branch when a corresponding remote ref exists.

Success is not just "command exited zero". It means:

- no unmerged files remain
- the target branch tree matches the source branch tree after the squash
  commit
- the squash commit exists on the target branch
- the local source branch has been deleted
- the remote source branch has been deleted when it previously existed
- the repo is left in the expected state for the chosen flow

## Planned stage validation shape

| Stage | Validation intent | Preferred mechanism |
| --- | --- | --- |
| `specify` | required artifacts exist and are not untouched template output | command `verify:` + file/content checks |
| `plan` | required plan artifacts exist; optional artifacts stay optional; plan is materially filled in | command `verify:` + targeted content checks |
| `tasks` | `tasks.md` exists, differs from template, and is structurally usable | command `verify:` or named `verify` step |
| `analyze` | findings output is parsed and summarized; blocking severities are named checks | named `verify` step after `analyze` |
| `implement` | implement iterations must make task progress; stage completion still requires all tasks checked off | named `verify` step after `implement` + artifact-based stage recognition |
| `code-review` | blocking finding threshold is explicit and inspectable | named `verify` step after review |
| `mergeback` | rebase completed, squash-merge produced the expected target tree, and local/remote branch deletion succeeded | named `verify` step after mergeback |

## Development loop: demo project + sample spec

We should dogfood this work with a **small demo project** and one
canonical sample spec.

Recommended working loop:

1. Create a fresh branch for the next full-lane experiment.
2. Run the full inner loop end to end on the demo project.
3. Inspect:
   - stage outputs
   - verification results
   - artifact-derived lane state
   - lane journal audit trail
   - review findings
   - mergeback behavior
4. Record what was wrong or unclear.
5. Fix the orchestration.
6. Repeat on a new fresh branch.

This loop is important because package-level tests alone will not tell us
whether the workflow feels correct and behaves correctly end to end.

## Recommended implementation order

1. Define the single-spec stage contract.
2. Replace ad hoc checks with DSL-native validation blocks.
3. Add the lane journal as audit data.
4. Make recovery deterministic from artifact truth + verification state.
5. Tighten the analyze/remediation loop to one remediation pass.
6. Tighten the implement/code-review loop.
7. Harden mergeback and interrupted-mergeback recovery.
8. Add inspect/status surfaces from artifact truth.
9. Repeat the demo-project test-fix loop until the lane is reliable.

## Exit criteria

The inner loop is "rock-solid" when all of the following are true:

- every stage has explicit validation expressed through the lane DSL
- resume decisions come from artifact truth + verification state, not from
  manifest flips or journal labels
- loop progress is durably visible
- mergeback has verified postconditions
- inspect/status can explain the last stop point clearly
- repeated fresh-branch demo-project runs complete with the expected
  artifacts and behavior end to end

## Current proof status

The lane has now been proven against a real Copilot-authored run:

- interrupted run recovery resumed from the artifact-derived stage instead
  of restarting from the coarse saved runner position
- analyze produced real `HIGH` / `CRITICAL` findings and triggered exactly
  one `tasks` remediation pass using the captured analyze output
- the run then progressed into `implement -> code-review`
- implement/review advanced with bounded loop semantics and exited once
  review had no `CRITICAL` findings
- mergeback completed with verified tree equality, local branch deletion,
  and remote branch deletion treated as conditional when no matching
  remote ref existed

## Mergeback-specific validation notes

Because the desired mergeback is a **squash merge**, the old
ancestor-based verification is no longer sufficient.

The mergeback stage should instead validate:

- rebase completed without unresolved conflicts
- squash commit was created on the target branch
- `git diff --quiet <source> <target>` (or an equivalent tree-equality
  check) passes before branch deletion
- the local spec branch no longer exists
- the remote spec branch no longer exists when one existed before the
  mergeback started

If the existing DSL check kinds cannot express those cleanly, add
narrowly scoped git-oriented checks rather than moving mergeback
validation into ad hoc Go-only logic.
