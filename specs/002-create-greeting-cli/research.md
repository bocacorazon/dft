# Research: 002-create-greeting-cli

## Decision 1: Keep implementation in repository-root Go entrypoint
- **Decision**: Use the existing root-level `main.go` executable path for the greeting command.
- **Rationale**: The repository already follows a minimal single-binary layout; this avoids unnecessary structure for a one-command feature.
- **Alternatives considered**: Add a new CLI package or subdirectory (rejected as unnecessary complexity for this scope).

## Decision 2: Use standard library only
- **Decision**: Implement and validate the greeting with Go's standard library only (`fmt`, `testing`).
- **Rationale**: Feature requirements are satisfied without third-party dependencies, keeping setup and maintenance minimal.
- **Alternatives considered**: Introduce assertion or CLI helper libraries (rejected due to no functional need).

## Decision 3: Treat CLI output as the contract surface
- **Decision**: Define the external interface as exact stdout output for `go run .` and pass/fail behavior for `go test ./...`.
- **Rationale**: The user-facing behavior is command-line output, so contract clarity must focus on emitted text, exit code, and no extra output.
- **Alternatives considered**: Define API-style contracts (rejected because this feature exposes no HTTP/service interface).

## Decision 4: Validate behavior via pure function assertion
- **Decision**: Keep greeting text in a deterministic function and assert exact string match in a focused unit test.
- **Rationale**: This gives precise regression protection for the required literal output while staying simple.
- **Alternatives considered**: Snapshot or integration-only shell tests (rejected as heavier than needed for one output line).
