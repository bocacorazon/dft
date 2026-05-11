# Research: 004-go-hello-dft

## Decision 1: Keep command implementation at repository root
- **Decision**: Use the existing root-level `main.go` entrypoint as the greeting command surface.
- **Rationale**: The repository already has a minimal single-binary layout, so adding subpackages/directories would add unnecessary complexity.
- **Alternatives considered**: Introduce a dedicated `cmd/` tree (rejected for this one-command scope).

## Decision 2: Use Go standard library only
- **Decision**: Implement and validate behavior using only standard library packages (`fmt`, `testing`).
- **Rationale**: Requirements are fully satisfied without third-party dependencies, which keeps setup simple and local.
- **Alternatives considered**: External assertion/CLI helper libraries (rejected as unnecessary for exact string validation).

## Decision 3: Treat exact stdout as the external contract
- **Decision**: Define the user-facing contract around `go run .` stdout, stderr, and exit status.
- **Rationale**: This feature exposes a CLI behavior, so output determinism is the core integration surface for users and CI.
- **Alternatives considered**: API-style service contracts (rejected because the feature exposes no network interface).

## Decision 4: Validate greeting behavior with deterministic unit assertion
- **Decision**: Assert exact greeting text through a focused Go test run in `go test ./...`.
- **Rationale**: This catches regressions in content or formatting while keeping test maintenance low.
- **Alternatives considered**: Shell/integration-only snapshots (rejected as heavier than needed for one-line output behavior).
