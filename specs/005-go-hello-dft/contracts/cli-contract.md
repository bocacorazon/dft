# CLI Contract: Local Greeting Command

## Scope
Contract for local command execution and automated verification of greeting behavior.

## Command Interface
- **Run command**: `go run .`
- **Expected stdout**: exactly `hello, dft` followed by newline
- **Expected stderr**: empty on success
- **Expected exit code**: `0` on success

## Test Interface
- **Run tests**: `go test ./...`
- **Required behavior**: at least one test asserts greeting output exactly equals `hello, dft`

## Behavioral Rules
- Command must not prompt for input.
- Command must emit exactly one greeting line per execution.
- Tests must fail with a clear mismatch if greeting text or line count changes.

## Requirement Traceability

| Requirement | Contract Coverage |
|-------------|-------------------|
| FR-001 | Command Interface expected stdout defines `hello, dft`. |
| FR-002 | Command Interface + Behavioral Rules require exactly one line with newline. |
| FR-003 | Behavioral Rules forbid prompts/interactive input. |
| FR-004 | Test Interface requires `go test ./...` with greeting assertion. |
| FR-005 | Behavioral Rules require clear failure on text/line count mismatch. |
| FR-006 | Compatibility Notes scope behavior to local repository execution only. |

## Compatibility Notes
- Contract applies to local repository execution only.
- No network, persistence, or external service interactions are part of this feature contract.
