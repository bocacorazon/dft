<!--
Sync Impact Report
Version change: unversioned template -> 1.0.0
Modified principles:
- Template principle 1 placeholder -> I. Go-First Modular Core
- Template principle 2 placeholder -> II. Test-First Delivery (NON-NEGOTIABLE)
- Template principle 3 placeholder -> III. Zero Failing Tests Gate
- Template principle 4 placeholder -> IV. Deterministic Workflow and Auditability
- Template principle 5 placeholder -> V. Simplicity and Explicit Boundaries
Added sections:
- Go Technical Standards
- Development Workflow
Removed sections:
- Template placeholder comments and example-only guidance
Templates requiring updates:
- ✅ updated: .specify/templates/plan-template.md
- ✅ updated: .specify/templates/spec-template.md
- ✅ updated: .specify/templates/tasks-template.md
- ✅ updated: .github/agents/speckit.tasks.agent.md
- ✅ reviewed: .specify/extensions/git/commands/speckit.git.commit.md
- ✅ reviewed: .specify/extensions/git/commands/speckit.git.feature.md
- ✅ reviewed: .specify/extensions/git/commands/speckit.git.initialize.md
- ✅ reviewed: .specify/extensions/git/commands/speckit.git.remote.md
- ✅ reviewed: .specify/extensions/git/commands/speckit.git.validate.md
- ✅ reviewed: .github/copilot-instructions.md
- ✅ reviewed: docs/000-foundation/overview.md
Follow-up TODOs:
- None
-->
# Dark Factory Toolkit Constitution

## Core Principles

### I. Go-First Modular Core

Dark Factory Toolkit code MUST be implemented as a Go module with small,
cohesive packages and explicit boundaries between orchestration, storage,
agent adapters, and command-line surfaces. Business logic MUST live behind
interfaces at real external boundaries, such as git, sqlite, filesystem,
subprocesses, and agent invocations; interfaces MUST NOT be introduced only
for speculative abstraction. The engine MUST NOT embed an LLM SDK in core
workflow logic. This keeps the headless engine portable, testable, and
replaceable at its operational edges.

### II. Test-First Delivery (NON-NEGOTIABLE)

Every production behavior change MUST begin with a failing Go test that
describes the expected behavior before implementation begins. Work MUST
follow Red-Green-Refactor: write or update the test, observe the intended
failure, implement the smallest correct change, then refactor while keeping
the full suite green. Table-driven tests MUST be used for meaningful input
matrices. This rule is non-negotiable because the project is a workflow
supervisor whose correctness depends on repeatable, executable specifications.

### III. Zero Failing Tests Gate

No task, feature, branch, or release is complete while any repository test
fails, including failures that existed before the current change. A failing
baseline MUST be fixed before unrelated implementation work continues, or the
work MUST be reduced to the smallest change that restores the suite. The
minimum gate is `go test ./...`; features that add commands, storage,
subprocess execution, or integration boundaries MUST also include the relevant
unit, contract, or integration tests. This prevents new work from hiding
behind inherited instability.

### IV. Deterministic Workflow and Auditability

Workflow execution MUST be deterministic from recorded inputs, durable state,
and committed artifacts. Local state mutations performed by a step MUST be
auditable as exactly one engine-owned commit per completed step, while
remote-only steps MUST record their audit data in run artifacts. Agents and
tools MUST leave local changes unstaged for the engine to own commit
boundaries. Errors MUST be surfaced with enough context to resume, remediate,
or escalate; silent success-shaped fallbacks are prohibited.

### V. Simplicity and Explicit Boundaries

The default design MUST be the simplest implementation that satisfies the
accepted specification and the constitution. New dependencies, background
services, concurrency, plugin points, or persistent formats MUST be justified
in the implementation plan with the rejected simpler alternative. Public
commands MUST expose clear text or structured output contracts, and internal
packages MUST keep side effects at explicit adapter boundaries. This preserves
the ability to reason about, test, and audit the engine.

## Go Technical Standards

The project MUST use Go's standard toolchain as the default quality gate:
`gofmt` for formatting, `go test ./...` for correctness, and `go vet ./...`
when a package set exists and vet is applicable. Generated code MUST identify
its generator and MUST be reproducible from committed inputs. Storage access
MUST pass through ports such as repositories or stores; orchestration code MUST
NOT call git, sqlite, the filesystem, or subprocesses directly when an adapter
boundary exists. Configuration, flow definitions, and run artifacts MUST be
plain files or sqlite-backed state unless a plan justifies a stronger storage
requirement.

## Development Workflow

Plans MUST document the Go version, package layout, test strategy, and any
constitution violations before design work proceeds. Task lists MUST include
test tasks before implementation tasks for every user story and MUST include
a baseline `go test ./...` gate that fails the work if any preexisting test
fails. Implementation MUST stop on the first failing required test and fix the
root cause rather than continuing with unrelated changes. Reviews MUST verify
that tests were written first, all tests pass, package boundaries remain
explicit, and auditability requirements are preserved.

## Governance

This constitution supersedes conflicting project guidance, templates, and
agent instructions. Amendments MUST update this file, include a Sync Impact
Report, propagate required changes to dependent templates or guidance, and
identify any deferred follow-up. Reviewers MUST reject specs, plans, tasks, or
implementation work that violates a MUST-level principle unless a constitution
amendment is approved first.

Versioning follows semantic versioning. MAJOR increments are required for
backward-incompatible governance changes, principle removals, or redefinitions
that alter project obligations. MINOR increments are required for new
principles, new mandatory sections, or materially expanded guidance. PATCH
increments are required for clarifications, wording changes, and
non-semantic corrections. The ratification date records first adoption; the
last amended date records the date of the latest content change.

Compliance review is required at each Spec Kit gate. `/speckit.plan` MUST
populate the Constitution Check from these principles, `/speckit.tasks` MUST
generate mandatory test-first tasks, and `/speckit.implement` MUST halt until
the full required test suite is green. Manual overrides are not permitted for
the Zero Failing Tests Gate.

**Version**: 1.0.0 | **Ratified**: 2026-05-11 | **Last Amended**: 2026-05-12
