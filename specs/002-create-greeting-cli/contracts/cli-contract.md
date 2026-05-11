# CLI Contract: Local Greeting Command

## Scope
Contract for the local greeting command and its baseline automated verification.

## Command Interface
- **Run command**: `go run .`
- **Expected stdout**: exactly `hello, dft` followed by newline
- **Expected stderr**: empty on success
- **Expected exit code**: `0` on success

## Test Interface
- **Run tests**: `go test ./...`
- **Required behavior**: a test validates that greeting output exactly equals `hello, dft`

## Behavioral Rules
- Command must not prompt for input.
- Command must print one greeting line only (no prefix/suffix noise).
- Test failures must clearly indicate mismatch between expected and actual greeting text.

## Compatibility Notes
- Contract targets local repository execution only.
- No network, persistence, or external service behavior is part of this contract.
