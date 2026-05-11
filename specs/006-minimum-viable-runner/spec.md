# Feature Specification: Minimum Viable Runner

**Feature Branch**: `[006-minimum-viable-runner]`  
**Created**: 2026-05-11  
**Status**: Draft  
**Input**: User description: "Build increment 1 (Minimum Viable Runner) for dft in Go. Must include `dft submit <flow-file>` and `dft status <run-id>` CLI commands, minimal YAML flow parsing for `agent` steps only, Copilot subprocess adapter (`copilot -p ... --agent ...`), file-backed run metadata and step audit artifacts, and `capture: true` plus `export_as` support for downstream templating. Scope limits: local execution only, no sqlite yet, no worktrees/commit orchestration yet."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Submit and run an agent-only flow (Priority: P1)

As a CLI user, I can submit a local flow file and execute its agent steps so I can run a complete automation attempt and obtain a run ID for tracking.

**Why this priority**: Submitting and executing a flow is the core user value of the minimum viable runner; without it, no runs can be produced.

**Independent Test**: Run `dft submit <flow-file>` with a valid agent-only YAML flow and verify a run ID is returned, run metadata is created, and each executed step has an audit artifact directory.

**Acceptance Scenarios**:

1. **Given** a valid flow file containing only agent steps, **When** the user runs `dft submit <flow-file>`, **Then** the command starts a local run, executes steps in order through Copilot, and returns a run ID.
2. **Given** a completed run, **When** the user inspects `.dft/runs/<run-id>/`, **Then** run metadata exists and each step has artifacts under `.dft/runs/<run-id>/<step-id>/`.
3. **Given** a flow file with any non-agent step type, **When** the user runs `dft submit <flow-file>`, **Then** the command fails with a clear unsupported-step error and no partial successful run is reported.

---

### User Story 2 - Check run progress and final outcome (Priority: P2)

As a CLI user, I can query run status by run ID so I can understand whether a run is pending, running, succeeded, or failed.

**Why this priority**: Once runs exist, users need a stable status command to monitor and troubleshoot outcomes.

**Independent Test**: Run `dft status <run-id>` for existing and non-existing run IDs and verify the command returns accurate state and clear errors.

**Acceptance Scenarios**:

1. **Given** a valid existing run ID, **When** the user runs `dft status <run-id>`, **Then** the command reports the current run state and step-level outcome summary from stored metadata.
2. **Given** an unknown run ID, **When** the user runs `dft status <run-id>`, **Then** the command exits with a clear "run not found" message.

---

### User Story 3 - Reuse captured output in downstream prompts (Priority: P3)

As a flow author, I can capture a step's output and export it for later templating so downstream steps can reuse prior results without manual copy/paste.

**Why this priority**: Captured output chaining is the minimum capability needed for multi-step value beyond isolated single prompts.

**Independent Test**: Submit a two-step flow where step 1 sets `capture: true` and `export_as`, then verify step 2 receives the exported value through templating and produces expected behavior.

**Acceptance Scenarios**:

1. **Given** an agent step with `capture: true` and `export_as`, **When** the step completes, **Then** its captured output is stored and made available to later steps by the export name.
2. **Given** a later agent step template referencing an exported value, **When** that step executes, **Then** template variables are resolved using values captured from prior steps in the same run.
3. **Given** a template references an export name that was never produced, **When** execution reaches that step, **Then** the run fails with a clear missing-export error and records the failure in step artifacts.

### Edge Cases

- The submit command receives a path to a missing or unreadable flow file.
- The flow file is malformed YAML or omits required fields for one or more agent steps.
- A Copilot subprocess invocation exits unsuccessfully or returns no usable output.
- Two steps attempt to use the same `export_as` name in the same run.
- A step sets `export_as` without `capture: true`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide `dft submit <flow-file>` to start one local run from a user-provided flow file path.
- **FR-002**: The system MUST provide `dft status <run-id>` to read and report the state of a previously created run.
- **FR-003**: The submit workflow MUST accept YAML flows containing `agent` steps and MUST reject unsupported step types with explicit errors.
- **FR-004**: Each accepted run MUST receive a unique run ID and create file-backed run metadata under `.dft/runs/<run-id>/`.
- **FR-005**: The run metadata MUST include run-level lifecycle information (at minimum: run ID, submission time, current state, and final outcome).
- **FR-006**: For every executed step, the system MUST create step audit artifacts under `.dft/runs/<run-id>/<step-id>/`, including step input context, execution outcome, and captured command output when available.
- **FR-007**: Agent step execution MUST use the Copilot subprocess adapter command pattern `copilot -p ... --agent ...`.
- **FR-008**: Step execution MUST be local-only and MUST NOT depend on remote orchestrators, worktree automation, commit orchestration, or sqlite storage.
- **FR-009**: When `capture: true` is set on a step, the system MUST persist the step output for in-run reuse.
- **FR-010**: When `export_as` is provided with `capture: true`, the system MUST expose that captured value to downstream step templates by export name.
- **FR-011**: Downstream templating MUST resolve exported values from previously completed steps within the same run before invoking Copilot for the current step.
- **FR-012**: If downstream templating references a missing export, the system MUST mark the step and run as failed and record a clear diagnostic in step artifacts.

### Key Entities *(include if feature involves data)*

- **Flow Definition**: User-authored YAML document containing ordered `agent` steps with prompt content, optional capture settings, and optional export names.
- **Run Record**: Persisted run-level metadata for one `dft submit` invocation, including identity, timestamps, state transitions, and aggregate result.
- **Step Record**: Persisted execution data for one step within a run, including step ID, rendered prompt context, execution status, and audit outputs.
- **Export Context**: In-run map of named captured values produced by prior steps and consumed by downstream templating.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of valid agent-only flow submissions return a run ID and create a corresponding run directory with run metadata.
- **SC-002**: 100% of executed steps in successful runs produce step artifact directories containing auditable execution records.
- **SC-003**: In acceptance testing flows that use `capture: true` and `export_as`, at least 95% of downstream template resolutions succeed without manual intervention.
- **SC-004**: For known run IDs, users can retrieve current run status in under 5 seconds for at least 95% of status checks.
- **SC-005**: 100% of out-of-scope usage attempts (unsupported step type, missing export, unknown run ID) return explicit, actionable error messages.

## Assumptions

- Runs are executed by a single local user process and do not require distributed coordination.
- Flow authors provide step IDs that are unique within a flow file.
- Copilot CLI is available and authenticated in the local environment where commands are run.
- Increment 1 intentionally excludes sqlite-backed state, worktree lifecycle management, and commit orchestration features.
