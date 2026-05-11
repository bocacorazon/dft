# Feature Specification: Minimum Viable Runner Increment 1

**Feature Branch**: `[007-minimum-viable-runner]`  
**Created**: 2026-05-11  
**Status**: Draft  
**Input**: User description: "Build increment 1 (Minimum Viable Runner) for dft in Go. Must include `dft submit <flow-file>` and `dft status <run-id>` CLI commands, minimal YAML flow parsing for `agent` steps only, Copilot subprocess adapter (`copilot -p ... --agent ...`), file-backed run metadata under `.dft/runs/<run-id>/`, step audit artifacts under `.dft/runs/<run-id>/<step-id>/`, and `capture: true` with `export_as` support for downstream templating. Scope limits: local execution only, no sqlite yet, no worktrees/commit orchestration yet."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Submit and execute an agent flow (Priority: P1)

As a CLI user, I can submit a local flow file and run its agent steps so I can start an automation run and get a run ID to track.

**Why this priority**: Starting a run is the minimum value of the runner; all other capabilities depend on this.

**Independent Test**: Execute `dft submit <flow-file>` with a valid agent-only flow and confirm it returns a run ID and creates run/step artifacts.

**Acceptance Scenarios**:

1. **Given** a valid flow file with only agent steps, **When** the user runs `dft submit <flow-file>`, **Then** the system starts a local run, executes steps in order, and prints a run ID.
2. **Given** a flow file containing any unsupported step type, **When** the user runs `dft submit <flow-file>`, **Then** the command fails with an explicit unsupported-step error.
3. **Given** a submitted run, **When** the user inspects `.dft/runs/<run-id>/`, **Then** run metadata and step artifact directories are present.

---

### User Story 2 - Check run status by ID (Priority: P2)

As a CLI user, I can check the status of a previously submitted run so I can monitor progress and determine final outcome.

**Why this priority**: Once runs exist, users need a reliable way to inspect run state and troubleshoot failures.

**Independent Test**: Execute `dft status <run-id>` for an existing run and an unknown run ID and verify correct status/error behavior.

**Acceptance Scenarios**:

1. **Given** an existing run ID, **When** the user runs `dft status <run-id>`, **Then** the command returns the current run state and step outcome summary from stored metadata.
2. **Given** an unknown run ID, **When** the user runs `dft status <run-id>`, **Then** the command returns a clear run-not-found error.

---

### User Story 3 - Reuse captured outputs in later steps (Priority: P3)

As a flow author, I can mark step output for capture and export it by name so downstream step templates can reuse prior results automatically.

**Why this priority**: Output chaining is required for multi-step flows to produce cumulative value in the MVP.

**Independent Test**: Submit a two-step flow where step 1 uses `capture: true` and `export_as`, then verify step 2 template rendering uses the exported value.

**Acceptance Scenarios**:

1. **Given** a step configured with `capture: true` and `export_as`, **When** the step succeeds, **Then** the captured output is persisted and mapped to the export name for the same run.
2. **Given** a downstream step template referencing a prior export, **When** that step executes, **Then** the reference resolves to the captured value.
3. **Given** a downstream template references a missing export, **When** execution reaches that step, **Then** the step and run fail with a clear diagnostic recorded in step artifacts.

### Edge Cases

- The flow file path provided to `dft submit` does not exist or is unreadable.
- The flow file contains malformed YAML or missing required fields for an agent step.
- The Copilot subprocess invocation fails or produces unusable output.
- Multiple steps attempt to use the same `export_as` name within one run.
- A step sets `export_as` without `capture: true`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide `dft submit <flow-file>` to start one local run from a user-provided flow file.
- **FR-002**: The system MUST provide `dft status <run-id>` to report status for a previously submitted run.
- **FR-003**: The submit flow MUST parse YAML definitions for `agent` steps and MUST reject unsupported step types with explicit errors.
- **FR-004**: Agent step execution MUST call a Copilot subprocess adapter using the command shape `copilot -p ... --agent ...`.
- **FR-005**: Each accepted run MUST receive a unique run ID and persist run-level metadata under `.dft/runs/<run-id>/`.
- **FR-006**: Run metadata MUST include at least run ID, submission time, current state, and final outcome when complete.
- **FR-007**: For every executed step, the system MUST persist step audit artifacts under `.dft/runs/<run-id>/<step-id>/`.
- **FR-008**: Step audit artifacts MUST include rendered input context, execution result, and captured subprocess output when available.
- **FR-009**: When a step sets `capture: true`, the system MUST persist that step output for reuse in later steps of the same run.
- **FR-010**: When `export_as` is set with `capture: true`, the captured value MUST be exposed to downstream step templating by export name.
- **FR-011**: Before each downstream step executes, template references to exports MUST resolve from values captured earlier in the same run.
- **FR-012**: If a required export is missing during template resolution, the system MUST fail the step and run and record an actionable error in step artifacts.
- **FR-013**: This increment MUST support local execution only and MUST exclude sqlite storage, worktree orchestration, and commit orchestration.

### Key Entities *(include if feature involves data)*

- **Flow Definition**: A YAML document describing ordered `agent` steps, including prompt content and optional capture/export settings.
- **Run Record**: File-backed metadata for one submission, including identity, lifecycle timestamps, current state, and final outcome.
- **Step Record**: File-backed audit data for one run step, including step identity, rendered prompt context, execution status, and command outputs.
- **Export Context**: In-memory and persisted mapping of export names to captured step outputs for reuse by downstream templating within a run.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of valid agent-only submissions return a run ID and create a persisted run record.
- **SC-002**: 100% of executed steps create corresponding step audit artifact directories with auditable outcome data.
- **SC-003**: At least 95% of acceptance-test flows using `capture: true` and `export_as` complete downstream template resolution without manual intervention.
- **SC-004**: For known run IDs, at least 95% of `dft status` checks return run state in under 5 seconds.
- **SC-005**: 100% of out-of-scope or invalid usage cases (unsupported steps, unknown run IDs, missing exports) return explicit actionable errors.

## Assumptions

- Users run this increment on a local machine with CLI access to Copilot and filesystem write permissions in the project workspace.
- Flow definitions for this increment are intentionally limited to `agent` steps and sequential execution.
- Each step ID is unique within a single flow file.
- The MVP does not require sqlite, multi-machine coordination, worktree management, or commit lifecycle automation.
