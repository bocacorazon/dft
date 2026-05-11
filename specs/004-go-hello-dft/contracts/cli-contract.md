# CLI Contract: Local Greeting Command

## Scope
Contract for local command execution and automated verification of the greeting behavior.

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
- Tests must fail with a clear mismatch if greeting text changes or extra output is introduced.

## Compatibility Notes
- Contract applies to local repository execution only.
- No network, persistence, or external service interactions are part of this feature contract.
