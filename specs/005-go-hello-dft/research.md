# Research: 005-go-hello-dft

## Decision 1: Keep implementation at repository root
- **Decision**: Keep the greeting command exposed through the existing root-level `main.go` entrypoint.
- **Rationale**: The repository already uses a simple single-binary layout, so introducing package or directory reshaping would add complexity without user value.
- **Alternatives considered**: Introduce a dedicated `cmd/` tree (rejected for one-command scope).

## Decision 2: Use Go standard library only
- **Decision**: Implement and test behavior with standard library packages (`fmt`, `testing`) only.
- **Rationale**: Requirements are satisfied without external dependencies, keeping setup and execution local and deterministic.
- **Alternatives considered**: Third-party assertion or CLI helper libraries (rejected as unnecessary for exact string validation).

## Decision 3: Treat CLI stdout as the primary contract
- **Decision**: Define the interface contract around `go run .` stdout/stderr/exit code behavior.
- **Rationale**: This feature exposes user-facing command output, so deterministic output is the primary integration surface.
- **Alternatives considered**: Service/API contracts (rejected because no network interface exists in scope).

## Decision 4: Validate through the repository test workflow
- **Decision**: Require a focused greeting verification test in `go test ./...`.
- **Rationale**: This catches regressions in greeting content/line count while aligning with repository-standard test execution.
- **Alternatives considered**: Shell snapshot-only checks (rejected as heavier and less direct than a focused Go test).
