# Data Model: 002-create-greeting-cli

## Entity: GreetingCommand
- **Purpose**: Represents the local executable behavior exposed from the repository root.
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

## Entity: GreetingTest
- **Purpose**: Captures automated verification of the greeting behavior.
- **Fields**:
  - `test_name` (string, required, fixed: `TestGreeting`)
  - `invocation` (string, required, fixed: `go test ./...`)
  - `expected_value` (string, required, fixed: `hello, dft`)
  - `status` (enum: `pass|fail`)

## Relationships
- A `GreetingCommand` produces one `GreetingOutput` per execution.
- A `GreetingTest` validates the `GreetingOutput.text` value against requirements.

## Validation Rules
- `GreetingOutput.text` MUST exactly match `hello, dft` (FR-001, FR-003).
- `GreetingOutput.line_count` MUST be exactly `1` (FR-002).
- `GreetingCommand.expects_input` MUST remain `false` (FR-002).
- `GreetingTest` MUST be runnable through the default Go workflow `go test ./...` (FR-004).

## State Transitions
- `GreetingTest.status`: `fail -> pass` when output matches `hello, dft`; `pass -> fail` on any output drift.
