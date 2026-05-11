# Tasks: Local Greeting Command

**Input**: Design documents from `/specs/005-go-hello-dft/`  
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Tests are required for this feature (`FR-004`, `FR-005`), so test tasks are included in the user story phases.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: User story label (`[US1]`, `[US2]`) for story-phase tasks only
- Every task includes an exact file path

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish the local Go command surfaces and verification docs.

- [X] T001 Confirm Go 1.22 module target and root module metadata in `go.mod`
- [X] T002 Create/normalize root CLI entrypoint scaffold in `main.go`
- [X] T003 [P] Confirm manual execution steps for `go run .` and `go test ./...` in `specs/005-go-hello-dft/quickstart.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared command primitives required by all stories.

**⚠️ CRITICAL**: Complete this phase before starting any user story tasks.

- [X] T004 Define shared greeting constant and `greeting()` helper in `main.go`
- [X] T005 Align command behavior contract (stdout/stderr/exit code, no input) in `specs/005-go-hello-dft/contracts/cli-contract.md`

**Checkpoint**: Core command contract and shared primitives are ready.

---

## Phase 3: User Story 1 - Print greeting output (Priority: P1) 🎯 MVP

**Goal**: Running the root command prints exactly `hello, dft` once per execution.

**Independent Test**: From repository root, run `go run .` and confirm output is exactly `hello, dft` followed by one newline.

### Implementation for User Story 1

- [X] T006 [US1] Implement `main()` to print `greeting()` exactly once in `main.go`
- [X] T007 [P] [US1] Document deterministic repeated-run behavior in `specs/005-go-hello-dft/quickstart.md`
- [X] T008 [P] [US1] Update acceptance scenarios for command execution in `specs/005-go-hello-dft/spec.md`

**Checkpoint**: User Story 1 is runnable and independently verifiable from CLI output.

---

## Phase 4: User Story 2 - Validate with automated test (Priority: P2)

**Goal**: The repository test workflow catches any greeting output regressions.

**Independent Test**: Run `go test ./...` and verify greeting assertions pass; output drift causes a failing test.

### Tests for User Story 2

- [X] T009 [P] [US2] Add `TestGreeting` exact-text unit test for `greeting()` in `main_test.go`
- [X] T010 [US2] Add command execution helper for `go run .` in `main_integration_test.go`
- [X] T011 [US2] Add `TestGreetingCommandOutput` asserting exact stdout, empty stderr, and success exit in `main_integration_test.go`

### Implementation for User Story 2

- [X] T012 [P] [US2] Document automated verification expectations for greeting tests in `specs/005-go-hello-dft/quickstart.md`

**Checkpoint**: User Story 2 provides automated regression detection through standard `go test ./...`.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Final traceability and consistency cleanup across artifacts.

- [X] T013 [P] Cross-check requirement traceability between `specs/005-go-hello-dft/spec.md` and `specs/005-go-hello-dft/contracts/cli-contract.md`
- [X] T014 Verify final command/test usage examples and expected outputs in `specs/005-go-hello-dft/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies; start immediately.
- **Phase 2 (Foundational)**: Depends on Phase 1; blocks all user stories.
- **Phase 3 (US1)**: Depends on Phase 2; delivers MVP behavior.
- **Phase 4 (US2)**: Depends on Phase 2 and validates US1 behavior.
- **Phase 5 (Polish)**: Depends on completion of US1 and US2.

### User Story Dependencies

- **US1 (P1)**: Starts after foundational work; no dependency on other stories.
- **US2 (P2)**: Starts after foundational work; validates and protects US1 output contract.

### Within-Story Ordering

- For US2: add test helper before integration assertion (`T010` before `T011`).
- Keep each story independently testable before moving to polish.

---

## Parallel Opportunities

- **Setup**: `T003` can run in parallel with `T001`/`T002`.
- **US1**: `T007` and `T008` can run in parallel after `T006`.
- **US2**: `T009` and `T012` can run in parallel with `T010`/`T011` work.
- **Polish**: `T013` can run in parallel with `T014`.

---

## Parallel Example: User Story 1

```bash
# After implementing CLI output behavior:
Task: "Document deterministic repeated-run behavior in specs/005-go-hello-dft/quickstart.md"
Task: "Update acceptance scenarios for command execution in specs/005-go-hello-dft/spec.md"
```

## Parallel Example: User Story 2

```bash
# In parallel while integration contract test is being authored:
Task: "Add TestGreeting exact-text unit test in main_test.go"
Task: "Document automated verification expectations in specs/005-go-hello-dft/quickstart.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 (Setup).
2. Complete Phase 2 (Foundational).
3. Complete Phase 3 (US1) and validate with `go run .`.
4. Stop for MVP review/demo once US1 is independently confirmed.

### Incremental Delivery

1. Build shared baseline (Setup + Foundational).
2. Deliver US1 greeting output behavior.
3. Add US2 automated regression coverage.
4. Finish polish tasks for documentation and traceability consistency.

### Team Parallelization Strategy

1. One developer completes foundational command primitives (`T004`/`T005`).
2. Then split:
   - Developer A: US1 command behavior (`T006`) and US1 docs (`T007`/`T008`)
   - Developer B: US2 tests (`T009`-`T011`) and verification docs (`T012`)
3. Rejoin for polish tasks (`T013`/`T014`).
