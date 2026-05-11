# Data Model: 004-go-hello-dft

## Entity: GreetingCommand
- **Purpose**: Represents the executable command behavior exposed at repository root.
- **Fields**:
  - `command` (string, required, fixed: `go run .`)
  - `expects_input` (boolean, required, fixed: `false`)
  - `exit_code_on_success` (integer, required, fixed: `0`)

## Entity: GreetingOutput
- **Purpose**: Defines the exact stdout payload emitted by the command.
- **Fields**:
  - `text` (string, required, fixed: `hello, dft`)
  - `line_count` (integer, required, fixed: `1`)
  - `trailing_newline` (boolean, required, fixed: `true`)

## Entity: GreetingVerificationTest
- **Purpose**: Captures automated verification of greeting output behavior.
- **Fields**:
  - `test_name` (string, required, fixed: `TestGreeting`)
  - `invocation` (string, required, fixed: `go test ./...`)
  - `expected_output_text` (string, required, fixed: `hello, dft`)
  - `status` (enum: `pass|fail`)

## Relationships
- A `GreetingCommand` produces exactly one `GreetingOutput` per successful execution.
- A `GreetingVerificationTest` validates `GreetingOutput.text` against the required value.

## Validation Rules
- `GreetingOutput.text` MUST exactly match `hello, dft` (FR-001, FR-002).
- `GreetingOutput.line_count` MUST be exactly `1` (FR-002).
- `GreetingCommand.expects_input` MUST remain `false` (FR-003).
- `GreetingVerificationTest` MUST run in standard test workflow via `go test ./...` (FR-004, FR-005).

## State Transitions
- `GreetingVerificationTest.status`: `fail -> pass` when output matches required value; `pass -> fail` on any output/content drift.
