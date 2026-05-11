<!--
SYNC IMPACT REPORT
Version: 0.0.0 -> 1.0.0
Modified Principles:
  - New: I. Mandatory Test-Driven Development (TDD)
  - New: II. Zero Tolerance for Failing Tests
  - New: III. Idiomatic Go Practices
  - New: IV. Modularity & Clean Architecture
Added Sections: Quality Gates, Continuous Integration standards
Removed Sections: None (from template placeholders)
Templates requiring updates: 
  - .specify/templates/plan-template.md (⚠ pending)
  - .specify/templates/spec-template.md (⚠ pending)
  - .specify/templates/tasks-template.md (⚠ pending)
Follow-up TODOs: 
  - Ensure CI pipeline strictly fails on any Go test error.
-->
# Dark Factory Toolkit (dft) Constitution

## Core Principles

### I. Mandatory Test-Driven Development (TDD)
Test-Driven Development is strictly mandatory for all logic additions and behavioral modifications. 
- **Tests First:** Engineering begins with writing tests that validate the expected behavior. 
- **Red-Green-Refactor:** Tests MUST fail first (Red) to prove they are testing the intended behavior, then implemented to pass (Green), and finally refactored for clarity and performance while maintaining green status. Features without corresponding tests will be rejected.

### II. Zero Tolerance for Failing Tests
No code can be committed, pushed, or merged if any tests are failing. 
- **Scope of Rule:** This applies to the entire test suite, not just the code being modified. If pre-existing tests fail, it is the developer's immediate responsibility to either fix the broken test or restore the system to a passing state before proceeding with new work. Broken windows are not tolerated.

### III. Idiomatic Go Practices
We leverage Go's strengths by adhering to established idioms and conventions.
- **Simplicity:** Favor clear, readable code over clever, compact code. 
- **Standard Library:** Rely on the standard library (`stdlib`) whenever possible before introducing external dependencies.
- **Typing and Error Handling:** Explicitly handle all errors (`if err != nil`). Avoid panics for flow control. Ensure structs and interfaces are properly segregated.

### IV. Modularity & Clean Architecture
Every component within `dft` must be cleanly designed.
- **Self-Contained Packages:** Modules and libraries must be highly cohesive and loosely coupled.
- **Clear Interfaces:** Establish and adhere to clear contracts between internal packages (e.g., `gitx`, `runner`, `cli`, `store`).

## Quality Gates & Verification

No code is considered complete until it passes all predefined quality gates.
- **Local Verification:** Developers must run the full test suite via standard Go tooling (`go test ./...`) prior to any commit.
- **Test Coverage:** Code coverage should continuously improve or remain stable. New packages require comprehensive coverage of all branches and failure states.

## Development Workflow

The development process must systematically integrate TDD, validation, and design review.
1. **Spec & Plan:** Ensure feature intent is fully codified via Spec Kit workflows (`spec.md`, `plan.md`) before writing code.
2. **Acceptance Criteria Definition:** Clear acceptance criteria must be documented to guide the TDD test cases.
3. **Red-Green-Refactor Implementation:** As stated in Principle I.
4. **Peer/Automated Review:** Final verification of both the business logic and the test fidelity.

## Governance

This constitution supersedes all other documentation, practices, and individual preferences within the `dft` repository. 
- **Compliance:** All Pull Requests, manual or automated (via AI agents), MUST adhere to these rules. Any PR violating TDD or containing failing tests will automatically be closed or blocked.
- **Amendments:** Changing these rules requires a proposed draft in a branch, explicit justification in a pull request, and mutual agreement recorded in version history.
- **Version Bumping:** Major version for principle additions/removals; Minor for explicit expansions; Patch for clarifications.

**Version**: 1.0.0 | **Ratified**: 2026-05-11 | **Last Amended**: 2026-05-11
