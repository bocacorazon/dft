# Quickstart: 005-go-hello-dft

## Prerequisites
- Go 1.22 toolchain installed locally
- Repository checkout on branch `005-go-hello-dft`

## 1) Run planning setup
```bash
cd /home/marcos/Projects/dft
.specify/scripts/bash/setup-plan.sh --json
```
Expected key output paths:
- `FEATURE_SPEC=/home/marcos/Projects/dft/specs/005-go-hello-dft/spec.md`
- `IMPL_PLAN=/home/marcos/Projects/dft/specs/005-go-hello-dft/plan.md`

## 2) Run the greeting command
```bash
go run .
```
Expected stdout:
```text
hello, dft
```
Run it multiple times and confirm each run prints the same single line and exits successfully.

## 3) Run automated verification
```bash
go test ./...
```
Expected result: test execution includes `TestGreeting` and `TestGreetingCommandOutput`; both pass when output remains exactly `hello, dft` and fail on output drift.

## 4) Confirm generated planning artifacts
```bash
ls specs/005-go-hello-dft
```
Expected artifacts:
- `plan.md`
- `research.md`
- `data-model.md`
- `quickstart.md`
- `contracts/`

## 5) Validate agent context reference
Check `.github/copilot-instructions.md` and confirm the `SPECKIT START/END` block points to:
- `specs/005-go-hello-dft/plan.md`
