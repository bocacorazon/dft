# Dark Factory Toolkit — Dogfood Bootstrap Implementation Plan

> Companion to `docs/000-foundation/overview.md`. This plan turns the
> foundation design into an incremental implementation sequence that
> dogfoods dft as soon as the loop is testable.

## Problem and approach

The repository currently contains foundation design documentation and
Spec Kit/Copilot scaffolding, but no Go module or runnable `dft`
implementation. The goal is to bootstrap dft in increments so the tool
can begin producing and executing its own demand packages as early as
practical, while staying inside the project constitution: Go-first
modular core, test-first delivery, zero failing tests,
deterministic/auditable execution, and simple explicit boundaries.

The plan uses a deterministic stub adapter first, then adds real
Copilot CLI execution in the following increment. This gives us a
runnable dev loop that can be tested reliably before introducing
non-deterministic agent calls. Each increment must produce a working,
tested slice and feed observed execution results into the next
increment's demand package.

## Current state

- Tracked source is limited to `docs/000-foundation/overview.md`.
- Spec Kit assets exist in `.specify/` and `.github/agents/`, including
  `speckit.specify`, `speckit.plan`, `speckit.tasks`, and
  `speckit.implement` agents.
- The constitution requires Go, Red-Green-Refactor, `go test ./...`,
  explicit adapter boundaries, and auditable workflow execution.
- The foundation design defines the branch topology: create an
  increment branch from the repo default branch, create spec
  branches/worktrees from the increment branch, merge completed specs
  back into the increment branch, and merge the increment back to
  default only after final review.

## Assumptions and decisions

- **Language/toolchain**: Go module, initially standard library only
  unless a later increment justifies a dependency.
- **Agent execution**: deterministic stub adapter in the first runnable
  loop; real Copilot CLI adapter in the next increment after the
  execution contract is tested.
- **Flow format**: start with typed Go structs and built-in minimal
  flows; introduce external flow files only after the orchestration loop
  is stable.
- **Persistence**: start with filesystem artifacts and a small run
  journal; add sqlite-backed state when `status`, `inspect`, and
  `resume` need durable queryability.
- **Dogfood trigger**: as soon as intake + stub agents + orchestrator +
  verification can run end-to-end, use dft to generate the next demand
  package for dft itself.
- **Branch management**: implement branch/worktree orchestration before
  real dogfooding so every increment and spec has the topology described
  in the design.

## Target repository shape

```text
cmd/dft/
  main.go

internal/
  app/                 # top-level use cases: submit, status, inspect, resume
  domain/              # DemandPackage, Spec, WBS, Flow, Step, Run, Verdict
  ports/               # interfaces for git, filesystem, state, agents, verification
  adapters/
    agentstub/         # deterministic adapter fixtures
    copilot/           # real Copilot CLI subprocess adapter
    fs/                # artifact filesystem
    git/               # branch/worktree/merge implementation
    state/             # run journal, then sqlite store
    verify/            # command/file/json verification adapters
  orchestration/       # macro-loop and spec loop coordination
  flow/                # minimal flow runner
  intake/              # demand intake and package creation
  review/              # final review gate abstractions

.dft/
  agents/              # dft-owned agent prompts provisioned by init/sync
  flows/               # later externalized flows
  context/             # project context bundle
  runs/                # audit artifacts

tests/integration/     # CLI and end-to-end tests using temp repos
```

## Increments

### Increment 0 — Go scaffold and constitutional gate

Create the executable skeleton and test harness without implementing
orchestration behavior.

- Add `go.mod`, `cmd/dft`, and package directories.
- Add baseline CLI contract tests for `dft --help` and an empty
  `go test ./...` gate.
- Add test helpers for temp repos, fixture files, and fake clocks/IDs.
- Define package boundaries in code comments only where useful; avoid
  speculative interfaces until an adapter boundary exists.
- Validation: `go test ./...` passes and `dft --help` documents planned
  commands without pretending unfinished commands work.

### Increment 1 — Intake and deterministic demand-package generation

Deliver the first useful command: turn user intent into a demand package
using a deterministic agent adapter.

- Define `DemandPackage`, `AcceptanceCriterion`, `RunID`, and artifact
  references in `internal/domain`.
- Implement `dft submit --dry-run --adapter stub "<demand>"` or
  equivalent initial command that writes `.dft/runs/<run-id>/intent/`.
- Add `AgentAdapter` port and `agentstub` implementation that maps
  prompt + fixture name to structured output.
- Create initial dft-owned agents as markdown prompts in `.dft/agents/`:
  - `dft-intake.agent.md`: normalize raw demand into a demand package.
  - `dft-demand-package.agent.md`: refine acceptance criteria and
    assumptions.
- Record prompts, stub outputs, parsed demand package, and validation
  errors as audit artifacts.
- Validation: tests prove malformed agent output fails loudly, valid
  output creates a demand package, and no local mutation escapes the run
  artifact directory.

### Increment 2 — Branch/worktree manager and increment envelope

Make every run branch-aware before the tool starts changing itself.

- Add `GitPort` and `WorktreeManager` boundaries.
- Implement default-branch discovery, increment branch creation from
  default HEAD, spec branch creation from increment HEAD, merge spec
  branch back to increment, and final merge gate abstraction.
- Support deterministic tests with fake git plus integration tests in
  temporary git repositories.
- Encode the Spec Kit behavior discovered from `speckit.specify`: when
  invoking Spec Kit, dft must check out the intended increment base and
  pass `GIT_BRANCH_NAME` for exact per-spec branch names.
- Validation: tests prove branch topology and failure behavior without
  touching the user's active branch.

### Increment 3 — Minimal WBS, lane selection, and built-in flow runner

Create the smallest orchestration loop that can decompose a demand
package into specs and run one spec through a stubbed lane.

- Define `SpecRef`, `WBS`, `LaneAssignment`, `Flow`, `Step`, and `Run`
  domain types.
- Add stub-backed agents:
  - `dft-wbs-builder.agent.md`: produce an append-only DAG of specs.
  - `dft-lane-selector.agent.md`: assign `spec`, `streamlined`, or
    `manual` lanes.
- Implement a built-in flow runner for sequential `agent`, `function`,
  and `verify` steps using typed Go structs.
- Keep flow execution deterministic: no external flow DSL, no templating
  engine, and no raw shell yet.
- Add run monitoring data: step status, started/completed timestamps
  from injectable clock, captured stdout/stderr/artifacts, and
  structured errors.
- Validation: one demand package becomes one WBS, one spec
  branch/worktree is created, stub lane steps execute, and audit
  artifacts are complete.

### Increment 4 — Basic verification and evaluator feedback

Close the first build/eval/remediate loop with deterministic
verification.

- Add verification ports and initial checks: `file_exists`,
  `file_missing`, `command_exit_zero`, `grep_matches`, and
  `json_path_equals` where justified.
- Create verification/evaluation agents:
  - `dft-eval-plan-author.agent.md`: produce deterministic verification
    plan candidates.
  - `dft-code-review.agent.md`: review the increment diff and produce
    findings.
  - `dft-fix-planner.agent.md`: map failed checks/findings to a WBS
    amendment or child demand package.
- Implement evaluator execution after spec orchestration and before
  final merge.
- Feed results forward by writing `.dft/runs/<run-id>/evaluation.json`
  and including the previous run's findings in the next increment's
  intake context.
- Validation: failing checks block final merge, produce actionable
  findings, and can append a remediation spec to the WBS.

### Increment 5 — First real dogfood loop with stub agents

Use dft itself to generate the next dft demand package while still using
the deterministic adapter.

- Run `dft submit --adapter stub` against a real dft improvement request.
- Monitor the run with `dft status` and inspect artifacts with
  `dft inspect` if those commands exist; otherwise inspect the run
  directory directly for this increment only.
- Evaluate whether generated demand package, WBS, lane assignment, and
  verification plan are useful enough for implementation.
- Convert the observed gaps into the next demand package automatically
  or semi-automatically through the intake agent.
- Validation: the next implementation increment is planned from
  dft-produced artifacts, not hand-written from scratch.

### Increment 6 — Real Copilot CLI adapter

Replace the stub adapter in real runs while preserving deterministic
test coverage.

- Implement `copilot` adapter as a subprocess boundary with explicit
  argv, cwd, environment, timeout, transcript capture, and non-zero exit
  handling.
- Keep unit tests on the stub adapter; add integration tests that can be
  skipped unless Copilot CLI is available.
- Pass project context and agent file paths explicitly.
- Ensure agent output parsing is strict: malformed or missing structured
  output fails with context rather than falling back silently.
- Validation: a real Copilot-backed intake run can create a demand
  package and record full prompt/transcript/output artifacts.

### Increment 7 — Durable state, inspect/status/resume

Make dogfooding practical across crashes and longer runs.

- Add `RunStore` and `JobStore` ports.
- Introduce sqlite-backed `.dft/state.db` only after the file-journal
  behavior is understood; document why plain files are no longer
  sufficient.
- Implement `dft status`, `dft inspect`, `dft resume`, and `dft cancel`
  around durable run state.
- Reconcile state from committed step metadata and run artifacts after
  interruption.
- Validation: integration tests simulate a crash after a committed step
  and resume from the first incomplete step.

### Increment 8 — Final review and merge automation

Complete the increment lifecycle.

- Implement final increment diff review against the default branch.
- Support human approval gates and structured code-review findings.
- Add final merge implementation and remote-only audit records for
  push/PR operations when enabled.
- Keep default behavior local-only until remote operations are
  explicitly requested.
- Validation: final merge is impossible until all specs complete, Eval
  passes, and review approval is recorded.

### Increment 9 — Increase complexity deliberately

Only after the sequential dogfood loop works, expand capabilities.

- Externalize flow definitions into `.dft/flows/` with a minimal
  declarative format.
- Add bounded loops where review/remediation needs convergence.
- Add spec parallelism by raising the cap above one; rely on
  branch/worktree-per-spec topology already implemented.
- Add `dft init` and `dft sync` to provision/update `.dft/agents`,
  `.dft/context`, and default flows.
- Add GitHub PR creation/merge integration as remote-only audited steps.

## Dogfood operating loop

Each increment follows this loop:

1. Write or update the demand package for the increment.
2. Generate or refine WBS specs and lane assignments.
3. Create the increment branch from the default branch.
4. For each spec, create a spec branch/worktree from the current
   increment branch.
5. Run the assigned lane with test-first tasks.
6. Commit each local-mutating completed step through the engine boundary.
7. Merge successful spec branches back into the increment branch.
8. Run deterministic verification and evaluator review.
9. Feed findings into a WBS amendment or the next increment's demand
   package.
10. After all specs pass, run final review and merge increment to
    default.

## Agent set to create

- `dft-intake.agent.md`: converts raw user request into normalized demand
  package.
- `dft-demand-package.agent.md`: clarifies scope, acceptance criteria,
  assumptions, and non-goals.
- `dft-wbs-builder.agent.md`: decomposes a demand package into an
  append-only spec DAG.
- `dft-lane-selector.agent.md`: selects lane per spec with rationale.
- `dft-eval-plan-author.agent.md`: creates deterministic checks from
  acceptance criteria.
- `dft-code-review.agent.md`: reviews the increment diff and flags
  correctness/security/audit issues.
- `dft-fix-planner.agent.md`: converts eval/review findings into WBS
  amendments or child demand packages.
- `dft-merge.agent.md`: prepares final merge summary and verifies merge
  prerequisites.

## Test strategy

- Every production behavior starts with a failing Go test.
- Unit tests use fake clocks, fake IDs, fake filesystem/git/agent
  adapters where possible.
- Integration tests use temporary git repositories and real filesystem
  artifacts.
- Copilot integration tests are opt-in and skipped unless the CLI is
  available.
- Required gate after every task group: `go test ./...`; later add
  `go vet ./...` when packages exist.
- Tests must cover error paths: malformed agent output, missing files,
  failed git operations, failed verification, interrupted runs, and
  attempted merge before approval.

## Monitoring and feedback artifacts

- `.dft/runs/<run-id>/manifest.json`: run identity, demand package,
  increment branch, adapter, and status.
- `.dft/runs/<run-id>/<step-id>/prompt.md`: exact prompt sent to an
  agent.
- `.dft/runs/<run-id>/<step-id>/stdout.txt` and `stderr.txt`: raw
  adapter/tool output.
- `.dft/runs/<run-id>/<step-id>/parsed.json`: strict parsed result.
- `.dft/runs/<run-id>/wbs.json`: current append-only WBS.
- `.dft/runs/<run-id>/verification.json`: deterministic check results.
- `.dft/runs/<run-id>/evaluation.json`: review findings and suggested
  feedback.
- `.dft/runs/<run-id>/next-demand-package.json`: optional generated seed
  for the next increment.

## Risks and mitigations

- **Non-deterministic agents too early**: start with stub adapter and
  strict output contracts.
- **Overbuilding the DSL**: keep flows in Go structs until the first loop
  works.
- **Branch operations damaging user worktree**: test in temp repos, use
  worktrees, refuse dirty/ambiguous states with explicit errors.
- **Silent agent parse failures**: strict schemas and no success-shaped
  fallbacks.
- **Dogfood before verification is meaningful**: first dogfood is
  stub-only and must produce inspectable artifacts before real Copilot
  execution.
- **Dependency creep**: standard library first; justify sqlite and any
  parser dependencies only at the increment that needs them.

## Execution notes

- Do not implement all increments at once. Each increment must be
  independently tested, reviewed, and usable as input to the next.
- Prefer local-only behavior until remote steps are explicitly required.
- Keep generated/provisioned agent files versioned and auditable.
- Every branch, step, and run artifact should be traceable to a demand
  package, spec, and step ID.
