# Tasks: Minimum Viable Runner Increment 1

**Input**: Design documents from `/specs/007-minimum-viable-runner/`  
**Prerequisites**: plan.md (required), spec.md (required), checklists/requirements.md

**Tests**: No dedicated test-writing phase is included because the specification does not explicitly require a TDD workflow; each story still includes independent runtime validation criteria.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Task is parallelizable (different files, no dependency on incomplete tasks)
- **[Story]**: Story label for user-story phases only (`[US1]`, `[US2]`, `[US3]`)
- Every task includes a concrete file path

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish command, documentation, and package surfaces for the runner increment.

- [ ] T001 Align CLI usage text and subcommand dispatch contract in `main.go`
- [ ] T002 Create operator quickstart for submit/status local workflows in `specs/007-minimum-viable-runner/quickstart.md`
- [ ] T003 [P] Define submit/status and artifact contract expectations in `specs/007-minimum-viable-runner/contracts/runner-cli-contract.md`
- [ ] T004 [P] Create package scaffolds in `internal/cli/submit.go`, `internal/cli/status.go`, `internal/flow/types.go`, `internal/copilot/adapter.go`, `internal/runner/engine.go`, and `internal/store/filesystem.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core runner primitives that MUST exist before any user story implementation.

**⚠️ CRITICAL**: Complete this phase before starting user-story tasks.

- [ ] T005 Define shared run/step/export data models in `internal/run/types.go` and `internal/flow/types.go`
- [ ] T006 [P] Implement run metadata persistence primitives and run directory creation under `.dft/runs/<run-id>/` in `internal/store/filesystem.go`
- [ ] T007 [P] Implement Copilot subprocess adapter with `copilot -p ... --agent ...` command construction and output capture in `internal/copilot/adapter.go`
- [ ] T008 Implement shared domain errors (invalid flow, unsupported step, run not found, missing export) in `internal/errors/errors.go`
- [ ] T009 Implement flow validation helpers for required fields, supported `agent` step type, and unique step IDs in `internal/flow/validate.go`
- [ ] T010 Implement export-template resolution primitives for downstream step rendering in `internal/runner/template.go`

**Checkpoint**: Foundational runner capabilities are ready for story implementation.

---

## Phase 3: User Story 1 - Submit and execute an agent flow (Priority: P1) 🎯 MVP

**Goal**: Run `dft submit <flow-file>` for valid agent-only flows, return a run ID, and persist run/step artifacts.

**Independent Test**: Execute `dft submit <flow-file>` with a valid agent-only flow and verify run ID output, run metadata under `.dft/runs/<run-id>/`, and per-step artifact directories.

### Implementation for User Story 1

- [ ] T011 [US1] Implement YAML flow parsing into ordered agent steps in `internal/flow/parser.go`
- [ ] T012 [US1] Implement submit command argument parsing and flow-file readability errors in `internal/cli/submit.go`
- [ ] T013 [US1] Implement sequential run execution and lifecycle transitions in `internal/runner/engine.go`
- [ ] T014 [P] [US1] Implement step audit artifact persistence (`rendered-context.json`, `result.json`, `copilot-output.txt`) in `internal/store/artifacts.go`
- [ ] T015 [US1] Wire submit command path and run ID output in `main.go`
- [ ] T016 [P] [US1] Document submit success and unsupported-step failure walkthroughs in `specs/007-minimum-viable-runner/quickstart.md`

**Checkpoint**: User Story 1 is independently runnable and auditable.

---

## Phase 4: User Story 2 - Check run status by ID (Priority: P2)

**Goal**: Run `dft status <run-id>` to retrieve current run state and step outcome summary.

**Independent Test**: Execute `dft status <run-id>` for an existing run and for an unknown run ID; verify state/summary output for known runs and explicit run-not-found errors for unknown IDs.

### Implementation for User Story 2

- [ ] T017 [US2] Implement run lookup and not-found behavior in `internal/store/filesystem.go`
- [ ] T018 [US2] Implement status projection from persisted run/step metadata in `internal/runner/status.go`
- [ ] T019 [US2] Implement `status` command handler and output formatter in `internal/cli/status.go`
- [ ] T020 [US2] Wire status subcommand routing and exit behavior in `main.go`
- [ ] T021 [P] [US2] Document known-run and unknown-run status command examples in `specs/007-minimum-viable-runner/quickstart.md`

**Checkpoint**: User Story 2 is independently testable against persisted run metadata.

---

## Phase 5: User Story 3 - Reuse captured outputs in later steps (Priority: P3)

**Goal**: Support `capture: true` and `export_as` for downstream templating across steps in one run.

**Independent Test**: Submit a two-step flow where step 1 captures/exports output and step 2 consumes it in templating; verify missing-export references fail the step/run with diagnostic artifacts.

### Implementation for User Story 3

- [ ] T022 [US3] Enforce capture/export validation rules (`export_as` requires `capture: true`, reject duplicate exports) in `internal/flow/validate.go`
- [ ] T023 [US3] Implement export context state tracking for run execution in `internal/runner/context.go`
- [ ] T024 [US3] Implement downstream prompt template rendering with export substitution in `internal/runner/template.go`
- [ ] T025 [US3] Persist captured outputs and missing-export diagnostics in `internal/store/artifacts.go`
- [ ] T026 [US3] Integrate capture/export resolution and failure propagation in `internal/runner/engine.go`
- [ ] T027 [P] [US3] Document capture/export authoring and missing-export failures in `specs/007-minimum-viable-runner/quickstart.md`

**Checkpoint**: User Story 3 is independently testable for export chaining and failure diagnostics.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final consistency, traceability, and operator-facing clarity across stories.

- [ ] T028 [P] Align requirement-to-implementation traceability between `specs/007-minimum-viable-runner/spec.md` and `specs/007-minimum-viable-runner/contracts/runner-cli-contract.md`
- [ ] T029 Normalize user-facing error phrasing across `internal/cli/submit.go` and `internal/cli/status.go`
- [ ] T030 [P] Finalize artifact layout examples and command contracts in `specs/007-minimum-viable-runner/contracts/runner-cli-contract.md`
- [ ] T031 Add end-to-end local execution walkthrough for all user stories in `specs/007-minimum-viable-runner/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies.
- **Phase 2 (Foundational)**: Depends on Phase 1 and blocks all user stories.
- **Phase 3 (US1)**: Depends on Phase 2 and delivers MVP submit/execute flow.
- **Phase 4 (US2)**: Depends on Phase 2; relies on persisted run metadata produced by compatible submit runs.
- **Phase 5 (US3)**: Depends on Phase 2; extends submit execution behavior with capture/export templating.
- **Phase 6 (Polish)**: Depends on completion of targeted user stories.

### User Story Dependency Graph

- **Recommended incremental order**: `US1 (P1) -> US2 (P2) -> US3 (P3)`
- **Parallelization option after Phase 2**: `US2` and `US3` can be developed in parallel lanes, then integrated on top of the US1 execution baseline.

### Within-Story Ordering

- Parse/validate before command wiring.
- Persisted model/store logic before CLI presentation logic.
- Story documentation updates after core behavior is implemented.

---

## Parallel Opportunities

- **Setup**: `T003`, `T004`
- **Foundational**: `T006`, `T007`
- **US1**: `T014`, `T016`
- **US2**: `T021`
- **US3**: `T027`
- **Polish**: `T028`, `T030`

---

## Parallel Example: User Story 1

```bash
Task: "Implement step audit artifact persistence in internal/store/artifacts.go"   # T014
Task: "Document submit success/failure walkthroughs in specs/007-minimum-viable-runner/quickstart.md"   # T016
```

## Parallel Example: User Story 2

```bash
Task: "Implement status projection in internal/runner/status.go"   # T018
Task: "Document status command examples in specs/007-minimum-viable-runner/quickstart.md"   # T021
```

## Parallel Example: User Story 3

```bash
Task: "Implement export template rendering in internal/runner/template.go"   # T024
Task: "Document capture/export failure cases in specs/007-minimum-viable-runner/quickstart.md"   # T027
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup (Phase 1).
2. Complete Foundational prerequisites (Phase 2).
3. Complete User Story 1 (Phase 3) and validate `dft submit <flow-file>`.
4. Pause for MVP review/demo.

### Incremental Delivery

1. Build shared base (Phases 1-2).
2. Deliver US1 submit/execution path.
3. Deliver US2 status visibility.
4. Deliver US3 capture/export templating.
5. Finish with cross-cutting polish tasks.

### Parallel Team Strategy

1. Team completes Setup + Foundational together.
2. Split by story lanes:
   - Engineer A: US1 submit/execution
   - Engineer B: US2 status reporting
   - Engineer C: US3 capture/export templating
3. Rejoin for polish and traceability tasks.
