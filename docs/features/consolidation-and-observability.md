# Consolidation, resume unification, and observability

Status: planned (W1→W5, sequenced)

This document evaluates the current dft implementation and lays out a sequenced plan to
remove legacy code, consolidate onto a single two-layer orchestration model, unify resume
on artifact truth, and add per-agent observability. It is judged on the current state of
the code, not the implementation history.

---

## Evaluation (design review)

Goal of dft: execute development jobs through a structured, verifiable, resumable workflow.

### Verdict

The core is sound, and the differentiating idea — **artifact-truth verification driving
resume** — is genuinely good and already implemented for the SpecKit lane. The main drag is
**internal duplication** (two orchestration stacks + two domain model sets live at once)
and a **god-file runner**, which is what makes the code feel "dense." None of that is
architectural rot; it is consolidation debt. Two things are also genuinely inconsistent
(two different resume strategies) and one thing is essentially absent (structured per-agent
observability).

### Strengths (keep these)

- **Closed-set, declarative verification.** 12 deterministic check kinds
  (`internal/domain/verification.go:13-29`) attached to steps/stages as `verify:` blocks.
  Inspectable, reusable for gating + resume + status. This is the best part of the design.
- **Artifact-truth resume (SpecKit lane).** `DecideSpecKitLaneResume`
  (`internal/orchestration/speckit_artifact_state.go:37-215`) derives progress from real
  files (spec.md/plan.md/tasks.md + parsed findings), not a journal. Exactly right, and it
  is what a linear `command`/`gate` YAML cannot do.
- **Clean execution-first boundary.** "Design upstream, dft executes frozen artifacts"
  (`docs/000-foundation/overview.md`) keeps dft decoupled from the design agents.
- **Lane is externalized correctly.** `.dft/flows/spec-lane.yaml` is provisioned and loaded
  if present; the embedded YAML is a `go:embed` default fallback (`provision.go`,
  `speckit_lane.go`). Good pattern.
- **Model-tier abstraction** (`xhigh/high/medium/low` resolved at runtime) keeps flows
  provider-agnostic.

### Q1 — Move code-review & mergeback to the Hermes side?

**No. Keep them in dft.**

- They are **build-time quality gates**, not spec-stage design. They consume the live
  worktree/git state and produce **structured decisions the run loop consumes** —
  `code-review` returns a `ReviewDecision{approved, findings}` that gates the
  implement/review loop (`speckit_lane.go:252-270`); `mergeback` resolves rebase conflicts
  whose success is then verified by git checks (`speckit_lane.go:305-349`).
- dft "owning the quality of what it builds" is the correct ownership line. Moving these to
  Hermes would split the feedback loop across two tools and force Hermes to reach into the
  execution worktree — breaking the design/execution separation the rest of dft maintains.
- The *prompts* for these agents are provisioned `.agent.md` assets, so they stay easy to
  version and tune without changing where they run.

### Q2 — Simplify / make the DSL more elegant?

The DSL *shape* (steps typed command/agent/gate/tool/function/verify/workflow/loop) is
reasonable and gives the verify/loop power the linear model lacks. The density is in the
**Go implementation**, not the YAML surface:

- **`Step` is a god-struct** (~8 mutually-exclusive shapes in one type). No invariant
  enforces "exactly one shape." Validate exactly-one-of at load, and/or split per-type
  payloads.
- **`runner.go` is ~1700 lines** with a 400+ line `executeFunctionStep` holding hardcoded
  dft-specific `function` subcommands. A `function` **handler registry**
  (`map[string]FunctionHandler`) + a split runner fixes this.
- **`function` steps are an escape hatch** carrying dft lifecycle logic as data-shaped Go.
  Either formalize a small handler registry or pull that logic back into typed stages — the
  current middle ground is the densest spot.

### Q3 — Observability surface for per-agent stats

**Today: essentially none structured.** Only free-text transcripts; `AgentResponse` carries
only `Raw string` (`internal/ports/agent.go`). No duration, tokens, model, or success/retry
captured as data. Copilot transcripts are keyed by agent name only, so repeat calls can
overwrite. Recommended surface is detailed in **W4**.

### Cross-cutting issues (current state)

1. **v1/v2 coexistence (top simplification lever).** Two orchestration stacks are both
   wired: v1 `MacroOrchestrator`, v2 `orchv2` dispatcher/executor; plus two domain model
   sets (`internal/domain` vs `internal/domain/v2`). v2 is a thin partial layer, not a full
   replacement. Pick one direction and delete the other.
2. **Two resume strategies that disagree.** Lane resume is artifact-truth
   (`speckit_artifact_state.go`); **build resume trusts `orchestration-result.json` status**
   and does not re-derive from artifacts (`build_command.go:116-139`). Unify on artifact
   truth.
3. **Three progress sources.** Artifact truth, `orchestration-result.json`, and the SQLite
   `steps` table all track progress; lane resume ignores the SQLite table. Collapse to one
   authoritative source (artifact truth) + an audit-only journal.
4. **Verification heuristics can false-pass.** `concreteMarkdownFile` only checks non-empty;
   `looksTemplated` is string-match (`speckit_artifact_state.go:253-304`); little checksum
   use. Tighten the highest-value checks.

---

## Resolved architecture direction

> Inner loop runs the whole flow and owns its own state (with a hook the orchestrator calls
> to ask "are you complete?"). Outer loop understands the DAG and dispatches the inner flow
> to whichever executor type is selected.

### Corrected premise (verified against the code)

An earlier draft assumed `orchv2` was the live survivor. The caller graph says otherwise —
**three generations coexist**, and the live CLI uses the newest:

| Gen | Packages | Wired to | Status |
|-----|----------|----------|--------|
| v3 (current) | `internal/execution` (`Service` inner + `Orchestrator` outer) | live `build`, `submit` | **survivor** |
| v2 | `internal/orchestration/v2` (orchv2: `Dispatcher` + executor registry + `SpeckitExecutor`), parts of `internal/domain/v2` | **only** `runBuildLegacy` | delete |
| v1 | `internal/orchestration` (`MacroOrchestrator`, `SpecPlanner`) + `internal/intake` + `internal/review` (FinalReviewer/FixPlanner/EvalPlanAuthor) + `eval.SurfaceContractAuthor` | **only** `runSubmitLegacy`/`runFullProcessLoop` | delete |

Further corrections to the evaluation above:
- **Artifact-truth resume is test-only.** `DecideSpecKitLaneResume`/`ResumeSpecKitLane` have
  **no** non-test callers. The capability is real but unwired.
- **The speckit lane is data, not code.** The full lane — specify→plan→tasks→analyze→
  implement→**code-review loop**→**mergeback** (with git verify checks) — lives in the
  **provisioned YAML** (`SpecKitLaneFlowYAML()` → `.dft/flows/spec-lane.yaml`) and is run by
  the generic `flow.Runner`. orchv2's Go `SpeckitExecutor` only adds **per-spec git worktree
  isolation** + the executor-type registry on top of that YAML.
- **Live contracts survive regardless:** `schema`/`validate` use `domainv2.SolutionDesign`
  and `domainv2.DemandPackage`; `status`/`inspect` use `loadSolutionDesign` /
  `loadOrchestrationResult` (domainv2); `evaluate`/`eval-harness` use `eval.Orchestrator`.

### Decision: Option C — unify onto one two-layer model (survivor = `internal/execution`)

- **Survivor = `internal/execution`** (`Service` inner + `Orchestrator` outer), the stack the
  live commands already use.
- **Fold in the good bits from orchv2 before deleting it:** an **executor-type registry** so
  the outer `Orchestrator` dispatches each spec to a selected executor; **per-spec git
  worktree** setup (ported from `SpeckitExecutor.buildWorktree` + `SpecWorktree`); and a
  **`Status()`** completeness hook on the inner executor that derives progress from artifact
  truth (port/rewire `DecideSpecKitLaneResume`).
- **Delete v1 and v2 entirely:** orchv2, `MacroOrchestrator`, `SpecPlanner`, `internal/intake`,
  `internal/review` (orphaned once Macro is gone), `eval.SurfaceContractAuthor`.
- **Keep code-review/mergeback** — they live in the provisioned lane YAML (Q1 stands).
- **No back-compat shims.** dft is pre-release; legacy paths, aliases, and deprecated flags
  are removed outright (clean breaks).
- **Increment-level final-review + remediation (`internal/review`) is deleted as legacy**, not
  rehomed. It is wired only to `MacroOrchestrator`, no live command exposes it, and per-spec
  code-review/mergeback in the lane already gives dft ownership of build quality. A finalize
  phase can be re-added later as a clean feature if wanted.

---

## W1 — Remove ALL legacy code & consolidate onto `internal/execution`

Foundational sweep: *"remove any legacy code from the codebase so there is no more
confusion."* No back-compat shims survive.

### Confirmed legacy inventory (to delete/rename)

- **Legacy CLI paths:** `runSubmitLegacy` + `runFullProcessLoop` + `runDogfoodFeedbackLoop`
  + `shouldUseLegacySubmit` (`cli.go`, `execution_commands.go`); `runBuildLegacy` +
  `shouldUseLegacyBuild` + its private helpers `writeOrchestrationResult`/`printBuildStatus`
  (`build_command.go`); the `--full`/`--dogfood` deprecation warning (`cli.go`).
  **Keep** `loadSolutionDesign` + `loadOrchestrationResult` (used by live `status`/`inspect`/
  `evaluate`).
- **v1 stacks:** `internal/orchestration/macro.go`, `internal/orchestration/spec_planner.go`,
  `internal/intake/`, `internal/review/`, `eval.SurfaceContractAuthor`.
- **v2 stack:** `internal/orchestration/v2/` (orchv2). Port worktree + speckit execution into
  `internal/execution` first.
- **Duplicate domain:** consolidate `internal/domain/v2` into `internal/domain`; the two
  `DemandPackage` types (`domain/demand.go`, `domain/v2/intent.go`) collapse to one
  `IncrementPackage` (keep the rich v2 shape; delete the simple one — its only live use is
  `domain.DemandPackage{ID: runID}` in `evaluate_command.go`).
- **"demand" naming:** Go identifiers (`DemandPackage`, `DemandPackageID`, `RawDemand`,
  `RefinedDemand`, `CreateDemandPackage`, …), JSON tag `raw_demand` (`run.go`), and the
  `demand-package` schema/validate alias (`schema_commands.go`).
- **Dead code:** `FromLegacyPlan` (`eval/legacy.go`); `legacyDefaultProvisionedAssets`
  (`provision.go`); the hermes dead placeholders (`hermes.go`); the legacy `runStatus`
  fallback in `showStatusV2`. **Preserve** `ToVerificationResult` if still used (rehome it out
  of the "legacy" file).

### Steps

1. **Delete the legacy CLI entrypoints** and stop `runBuild`/`runSubmit` from branching to
   them; remove `shouldUseLegacy*` and the deprecation warning.
2. **Port worktree + speckit execution into `internal/execution`.** Introduce an `Executor`
   interface (`Execute` + `Status`), make `Service` the default flow executor, add a
   per-spec git-worktree-capable executor (ported from `SpeckitExecutor`), and an
   executor-type **registry** the `Orchestrator` selects per lane/spec.
3. **Add the `Status()` hook** deriving completeness from artifact truth (port/rewire
   `DecideSpecKitLaneResume`); this is the seam W3 builds on.
4. **Delete v1 + v2 stacks:** orchv2, `MacroOrchestrator`, `SpecPlanner`, `internal/intake`,
   `internal/review`, `eval.SurfaceContractAuthor`, plus the now-dead helpers.
5. **Consolidate to one domain package:** fold the still-needed `domain/v2` types into
   `internal/domain`, delete `internal/domain/v2` and the simple `domain.DemandPackage`,
   rename to one `IncrementPackage`, fix all imports.
6. **Finish the demand→increment rename:** Go identifiers `Demand*`→`Increment*`, JSON tag
   `raw_demand`→`raw_increment`, drop the `demand-package` schema/validate alias so only
   `increment-package` remains. Update provisioned agent prompts/fixtures.
7. **Delete remaining dead code:** `FromLegacyPlan`, `legacyDefaultProvisionedAssets`, hermes
   placeholders. Preserve/rehome `ToVerificationResult`. **Status fallback decision (revised):**
   `runStatus`/`runInspect`/`updateRunStatus` are the live *manifest-based* status path for
   `build`/`submit` runs (which emit `execution-result.json`, not intent/solution artifacts),
   while `showStatusV2`/`printInspectV2`/`updateRunStatusV2` are the *phase-based* path. These
   are two live mechanisms, not legacy duplication — the `V2` suffix is meaningful
   disambiguation. The `runStatus` fallback is **kept**; fully unifying the two status sources
   is deferred to **W3** (resume/status unification). Only the misleading "legacy" comments were
   reworded.
8. **Verification gate (definition of done for W1):**
   - `grep -rniE 'legacy|deprecat|demand|orchestration/v2|shouldUseLegacy' internal/ cmd/ --include='*.go'`
     returns only intentional, non-legacy hits (ideally zero).
   - No `internal/orchestration/v2`, `internal/orchestration/macro.go`, `internal/intake`,
     `internal/review`; no `internal/domain/v2`; one `IncrementPackage`.
   - `go test ./...` green; both binaries rebuilt; `dft schema`/`validate` expose only
     `increment-package` (no `demand-package`); `build`/`submit` still run their lanes.

### W1a status — COMPLETE

Pure legacy removal + domain consolidation + demand→increment rename are done and verified:

- **Deleted stacks/dead code:** `internal/orchestration/v2` (orchv2), `internal/orchestration/macro.go` + `spec_planner.go`, `internal/intake/`, `internal/review/`, `eval/surface_author.go`, `eval/legacy.go` (`FromLegacyPlan` + the orphaned `ToVerificationResult`), legacy CLI entrypoints (`runSubmitLegacy`/`runBuildLegacy`/`runFullProcessLoop`/`runDogfoodFeedbackLoop`/`shouldUseLegacy*` + the `--full`/`--dogfood` warning), `legacyDefaultProvisionedAssets`, and the hermes dead placeholders.
- **Domain consolidated:** the rich `domain/v2` types folded into `internal/domain` as one `IncrementPackage` (plus `SolutionDesign`, `OrchestrationResult`, `SpecResult`, `TestPlan`, `TestScenario`, `AcceptanceCriterion`, `AmbiguityFinding`); `internal/domain/v2` and the simple `domain.DemandPackage` deleted; the stub + eval consumers migrated to the rich shape.
- **Rename done:** all `Demand*`→`Increment*` Go identifiers, the `Demand` agent-request/Step field → `Increment` (yaml/json `demand`→`increment`), the SQLite `raw_demand` column → `raw_increment`, and the `demand-package` schema/validate alias dropped (only `increment-package` remains).
- **Verified:** `go test ./...` green; the legacy/demand/v2 grep is CLEAN; both binaries rebuilt; `dft schema demand-package` and `dft validate demand-package` now return "unknown type".

**Deferred out of W1a (note vs. the original plan):** the `runStatus` fallback and the two
status mechanisms are intentionally retained (see step 7). The original W1 framing assumed a
single survivor stack; the discovered reality is that `internal/execution` (Service+Orchestrator)
is the live stack and the remaining "v1" speckit lane primitives in `internal/orchestration`
(lane/resume/worktree) are **live** (used by `resume` + `provision`), so they are kept and become
the source for the **W1b** executor fold-in.

### W1b status — COMPLETE

The two-layer model (inner loop owns flow + state, outer loop owns the DAG) is now realized
inside `internal/execution`:

- **`Executor` interface** (`internal/execution/executor.go`) — `Execute(...)` runs a spec's
  whole flow; `Status(...)` is the completeness hook the orchestrator calls to ask "are you
  complete?" before dispatching (artifact-truth resume-skip).
- **`ExecutorRegistry`** (same file) — `NewRegistry(default)` + `Register(lane, exec)` +
  `For(lane)`; an empty or unregistered lane resolves to the default executor.
- **Two executors:** `Service` is the default generic-flow executor (its `Status` reads
  `execution-result.json`); **`WorktreeExecutor`** (`internal/execution/worktree_executor.go`,
  lane `"spec"`) runs the spec-kit lane in a per-spec git worktree and derives `Status` from
  `orchestration.DecideSpecKitLaneResume` (artifact truth). The spec-kit lane stays **data,
  not code** — provisioned `.dft/flows/spec-lane.yaml` (embedded YAML fallback) run by the
  generic `flow.Runner`.
- **Outer loop is registry-driven** (`internal/execution/orchestrator.go`) — the dispatch
  loop selects `o.Registry.For(spec.Lane)` and calls `Status` before `Execute`, synthesizing a
  completed result for already-finished specs (DAG-level resume). `domain.SpecRef` gained a
  `Lane` field; `domain.ExecutionStatus` was added.
- **Live wiring:** `runSubmit` builds the registry and registers the worktree executor on the
  `spec` lane (`internal/app/execution_commands.go`); `runBuild` stays single-spec on the
  default `Service`.
- **Verified:** `go test ./...` green (incl. new `executor_test.go` covering registry
  resolution, `Service.Status`, orchestrator lane-selection + resume-skip, and worktree
  derivation/graceful Status); `go vet` clean; the legacy/demand/v2/orchv2 grep is CLEAN
  across all Go sources including tests; both binaries rebuilt; `dft schema increment-package`
  works and `dft schema demand-package` returns "unknown type".

This completes the W1 foundation. **W3** will promote `Executor.Status` to the single
resume/progress authority across build + submit (the status-mechanism unification deferred
out of W1a).

## W2 — Decompose the flow runner

1. Extract a `function`-step **handler registry** (`map[string]FunctionHandler`) and move
   the hardcoded subcommands out of `executeFunctionStep`.
2. Split `runner.go` → `runner.go` (Execute/run loop), `dispatch.go` (`executeStep` switch),
   `function_handlers.go`, `verify.go`, `loop.go`.
3. Enforce Step "exactly-one-shape" validation at load in `flow/loader.go`.
4. Pure refactor — no behavior change; tests stay green.

## W3 — Unify resume on artifact truth (depends on W1 Status hook)

1. Make `Executor.Status` (artifact truth) the single completeness source.
2. Rework build/submit resume to re-derive per-spec progress from `Executor.Status` instead
   of trusting `orchestration-result.json` status (`build_command.go:116-139`).
3. Demote `orchestration-result.json` to audit-only; reconcile or drop the unused SQLite
   `steps` table for resume.
4. Tests: interrupted single-build and DAG-submit both resume from the artifact-derived
   stage.

## W4 — Agent observability

1. Extend the port: `AgentResponse{ Raw string; Usage AgentUsage }`,
   `AgentUsage{ Model, InputTokens, OutputTokens, DurationMs, ExitCode, Attempts }`
   (`internal/ports/agent.go`).
2. Capture duration/exit/attempts at the `Invoke` chokepoint in copilot/hermes/stub adapters
   (wrap timing).
3. Tokens/model (best-effort, adapter-dependent): copilot → `--output-format json` + parse
   usage (`copilot.go:141`); hermes → parse usage / session store (`hermes.go:91-93`). Leave
   columns null when unavailable.
4. Persist `.dft/runs/<run>/agent-calls.jsonl` at the agent-step boundary; optionally an
   `agent_calls` SQLite table (`internal/adapters/state/sqlite_store.go`).
5. Add `dft stats <run-id>` (per-agent counts/tokens/duration) and fold into `dft inspect`.
6. Fix transcript overwrite: key copilot transcripts by run/step/attempt, not agent name
   (`copilot.go:182-206`).

## W5 — Tighten verification checks

1. Strengthen `concreteMarkdownFile` / `looksTemplated` with structural checks (required
   sections, min content, not-equal-to-template) in `speckit_artifact_state.go`.
2. Add checksum/integrity checks where they matter; reuse `file_checksum_differs`.
3. Add narrowly-scoped git checks for mergeback postconditions if not yet expressible.
4. Tests for the known false-pass cases.

---

## Dependencies & sequencing

- W1 → W2 → W3 (W3 needs W1's `Status` hook). W4 and W5 follow W3.
- Baseline + after each workstream: `go test ./...`.
- Rebuild both binaries after CLI-affecting changes:
  `go build -o ./bin/dft ./cmd/dft` and `go build -o "$(command -v dft)" ./cmd/dft`.
