# Tasks: Minimum Viable Runner

**Input**: Design documents from `/specs/006-minimum-viable-runner/`  
**Prerequisites**: plan.md, spec.md  

**Tests**: No dedicated test-writing tasks are included because the specification does not explicitly require a TDD or test-implementation workstream.  
**Organization**: Tasks are grouped by user story so each story can be implemented and validated independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Task is parallelizable (different files, no blocking dependency on incomplete tasks)
- **[Story]**: Story label required only in user-story phases (`[US1]`, `[US2]`, `[US3]`)
- Every task includes concrete file path(s)

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish command surfaces and feature documentation scaffolding.

- [ ] T001 Define subcommand entrypoint scaffold and usage banner in `main.go`
- [ ] T002 Create minimum viable runner usage scenarios in `specs/006-minimum-viable-runner/quickstart.md`
- [ ] T003 [P] Define submit/status and artifact contracts in `specs/006-minimum-viable-runner/contracts/runner-cli-contract.md`
- [ ] T004 [P] Create package skeleton files in `internal/cli/submit.go`, `internal/cli/status.go`, `internal/flow/types.go`, `internal/runner/engine.go`, and `internal/store/filesystem.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core primitives that block all story delivery until complete.

**⚠️ CRITICAL**: Complete this phase before starting user stories.

- [ ] T005 Implement shared run/step metadata models in `internal/run/types.go`
- [ ] T006 [P] Implement run ID generation, run-directory creation, and JSON persistence helpers in `internal/store/filesystem.go`
- [ ] T007 [P] Implement Copilot subprocess adapter with `copilot -p ... --agent ...` command construction and output capture in `internal/copilot/adapter.go`
- [ ] T008 Implement shared domain errors (unsupported step type, invalid flow, run not found, missing export) in `internal/errors/errors.go`
- [ ] T009 Implement base flow validation helpers (required fields and unique step IDs) in `internal/flow/validate.go`

**Checkpoint**: Foundations are complete; user stories can now proceed.

---

## Phase 3: User Story 1 - Submit and run an agent-only flow (Priority: P1) 🎯 MVP

**Goal**: Execute `dft submit <flow-file>` for valid agent-only flows and persist full run/step audit artifacts.

**Independent Test**: Run `dft submit <flow-file>` with a valid agent-only YAML; verify a run ID is returned, run metadata is present in `.dft/runs/<run-id>/`, and each step has an artifact directory.

### Implementation for User Story 1

- [ ] T010 [US1] Implement YAML parser for agent steps in `internal/flow/parser.go`
- [ ] T011 [US1] Implement submit command argument/file-loading path and unreadable-file errors in `internal/cli/submit.go`
- [ ] T012 [US1] Implement sequential run execution and lifecycle transitions in `internal/runner/engine.go`
- [ ] T013 [P] [US1] Implement per-step artifact writer (`input.json`, `result.json`, `copilot-output.txt`) in `internal/store/artifacts.go`
- [ ] T014 [US1] Wire `submit` command routing and run-ID output in `main.go`
- [ ] T015 [P] [US1] Document agent-only submit behavior and unsupported-step errors in `specs/006-minimum-viable-runner/quickstart.md`

**Checkpoint**: User Story 1 is independently runnable and auditable.

---

## Phase 4: User Story 2 - Check run progress and final outcome (Priority: P2)

**Goal**: Provide `dft status <run-id>` with accurate run state and step-level summary.

**Independent Test**: Run `dft status <run-id>` for known and unknown run IDs; verify state/summary output for known runs and explicit not-found errors for unknown runs.

### Implementation for User Story 2

- [ ] T016 [US2] Implement run lookup and not-found handling in `internal/store/filesystem.go`
- [ ] T017 [US2] Implement run/step status projection logic in `internal/runner/status.go`
- [ ] T018 [US2] Implement `status` command handler and formatter in `internal/cli/status.go`
- [ ] T019 [US2] Wire `status` subcommand routing and exit behavior in `main.go`
- [ ] T020 [P] [US2] Document status command examples and not-found behavior in `specs/006-minimum-viable-runner/quickstart.md`

**Checkpoint**: User Story 2 is independently testable from persisted run metadata.

---

## Phase 5: User Story 3 - Reuse captured output in downstream prompts (Priority: P3)

**Goal**: Support `capture: true` and `export_as` so downstream step templates can consume prior step outputs.

**Independent Test**: Submit a two-step flow where step 1 captures and exports output, step 2 templates it successfully; also verify missing export references fail with clear diagnostics and persisted failure artifacts.

### Implementation for User Story 3

- [ ] T021 [US3] Extend flow validation for `capture`/`export_as` constraints in `internal/flow/validate.go`
- [ ] T022 [US3] Implement export context state and duplicate `export_as` detection in `internal/runner/context.go`
- [ ] T023 [US3] Implement template rendering with export substitution before Copilot execution in `internal/runner/template.go`
- [ ] T024 [US3] Persist captured output payloads and missing-export diagnostics in `internal/store/artifacts.go`
- [ ] T025 [US3] Integrate capture/export/template failure handling in `internal/runner/engine.go`
- [ ] T026 [P] [US3] Document capture/export usage and missing-export failure scenarios in `specs/006-minimum-viable-runner/quickstart.md`

**Checkpoint**: User Story 3 is independently testable for export chaining and error handling.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final consistency, traceability, and operator-facing clarity.

- [ ] T027 [P] Align requirement-to-implementation traceability notes in `specs/006-minimum-viable-runner/spec.md`
- [ ] T028 Normalize user-facing error phrasing across submit/status commands in `internal/cli/submit.go` and `internal/cli/status.go`
- [ ] T029 [P] Finalize artifact-layout and command contract examples in `specs/006-minimum-viable-runner/contracts/runner-cli-contract.md`
- [ ] T030 Validate end-to-end command walkthrough text in `specs/006-minimum-viable-runner/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: Start immediately.
- **Phase 2 (Foundational)**: Depends on Phase 1; blocks all user stories.
- **Phase 3 (US1)**: Depends on Phase 2; delivers MVP submission/execution.
- **Phase 4 (US2)**: Depends on Phase 2 and uses persisted run data from US1-compatible runs.
- **Phase 5 (US3)**: Depends on Phase 2 and extends execution behavior introduced in US1.
- **Phase 6 (Polish)**: Depends on completion of targeted user stories.

### User Story Dependency Graph

- `US1 (P1) -> US2 (P2) -> US3 (P3)` for incremental rollout
- `US2` and `US3` can each begin after Phase 2 in parallel teams, but both integrate cleanest after US1 submit/run baseline exists

### Within-Story Ordering

- Parser/validation updates precede command wiring.
- Store/runner core logic precedes CLI output formatting.
- Story documentation updates occur after core implementation tasks.

---

## Parallel Opportunities

- **Phase 1**: `T003` and `T004` can run in parallel after `T001`.
- **Phase 2**: `T006` and `T007` can run in parallel while `T005` is in progress, then converge on `T008`/`T009`.
- **US1**: `T013` and `T015` can run in parallel after execution flow shape is established.
- **US2**: `T020` can run in parallel with `T017`/`T018`.
- **US3**: `T026` can run in parallel with `T023`/`T024` once validation semantics are fixed.
- **Polish**: `T027` and `T029` can run in parallel.

---

## Parallel Example: User Story 1

```bash
Task: "Implement per-step artifact writer in internal/store/artifacts.go"   # T013
Task: "Document agent-only submit behavior in specs/006-minimum-viable-runner/quickstart.md"   # T015
```

## Parallel Example: User Story 2

```bash
Task: "Implement run/step status projection logic in internal/runner/status.go"   # T017
Task: "Document status command examples in specs/006-minimum-viable-runner/quickstart.md"   # T020
```

## Parallel Example: User Story 3

```bash
Task: "Implement template rendering with export substitution in internal/runner/template.go"   # T023
Task: "Document capture/export scenarios in specs/006-minimum-viable-runner/quickstart.md"   # T026
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup (Phase 1).
2. Complete Foundational prerequisites (Phase 2).
3. Complete US1 (Phase 3) and validate `dft submit <flow-file>` end to end.
4. Pause for MVP validation/demo.

### Incremental Delivery

1. Build shared base (Phases 1-2).
2. Deliver US1 submit/run path.
3. Deliver US2 status visibility.
4. Deliver US3 capture/export templating.
5. Finish with polish and consistency tasks.

### Parallel Team Strategy

1. Team completes Phases 1-2 together.
2. Split by story lane:
   - Engineer A: US1 execution path
   - Engineer B: US2 status surface
   - Engineer C: US3 capture/export templating
3. Rejoin for polish tasks.
