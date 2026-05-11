# Quickstart: 002-create-greeting-cli

## Prerequisites
- Go 1.22 toolchain available locally
- Repository checked out with branch `002-create-feature-branch`

## 1) Run the greeting command
```bash
cd /home/marcos/Projects/dft
go run .
```
Expected output:
```text
hello, dft
```

## 2) Run automated verification
```bash
go test ./...
```
Expected result: test suite passes and includes greeting output validation.

## 3) Confirm planning artifacts
```bash
ls specs/002-create-greeting-cli
```
Expected artifacts:
- `plan.md`
- `research.md`
- `data-model.md`
- `quickstart.md`
- `contracts/`

## 4) Continue workflow
After planning completion, run `/speckit.tasks` to generate `tasks.md`.
