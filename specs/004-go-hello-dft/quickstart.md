# Quickstart: 004-go-hello-dft

## Prerequisites
- Go 1.22 toolchain installed locally
- Repository checkout on branch `003-go-hello-dft`

## 1) Run planning setup
```bash
cd /home/marcos/Projects/dft
.specify/scripts/bash/setup-plan.sh --json
```
Expected key output paths:
- `FEATURE_SPEC=/home/marcos/Projects/dft/specs/004-go-hello-dft/spec.md`
- `IMPL_PLAN=/home/marcos/Projects/dft/specs/004-go-hello-dft/plan.md`

## 2) Run the greeting command
```bash
go run .
```
Expected stdout:
```text
hello, dft
```

## 3) Run automated verification
```bash
go test ./...
```
Expected result: tests pass and include the greeting behavior check.

## 4) Confirm generated planning artifacts
```bash
ls specs/004-go-hello-dft
```
Expected artifacts:
- `plan.md`
- `research.md`
- `data-model.md`
- `quickstart.md`
- `contracts/`

## 5) Validate agent context reference
Check `.github/copilot-instructions.md` and confirm the `SPECKIT START/END` block points to:
- `specs/004-go-hello-dft/plan.md`
