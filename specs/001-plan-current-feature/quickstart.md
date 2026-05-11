# Quickstart: Generate plan artifacts for current feature branch

## Prerequisites
- Git branch: `001-plan-current-feature`
- Bash environment at repository root
- Speckit files present under `.specify/`

## 1) Run setup
```bash
cd /home/marcos/Projects/dft
.specify/scripts/bash/setup-plan.sh --json
```

Expected core outputs:
- `FEATURE_SPEC=/home/marcos/Projects/dft/specs/001-plan-current-feature/spec.md`
- `IMPL_PLAN=/home/marcos/Projects/dft/specs/001-plan-current-feature/plan.md`

## 2) Verify generated planning artifacts
```bash
ls specs/001-plan-current-feature
```
Expected files:
- `plan.md`
- `research.md`
- `data-model.md`
- `quickstart.md`
- `contracts/`

## 3) Validate agent context reference
Check `.github/copilot-instructions.md` and confirm the plan path between `SPECKIT START/END` points to:
- `specs/001-plan-current-feature/plan.md`

## 4) Optional: run project baseline tests
```bash
go test ./...
```

## 5) Continue workflow
After planning, run `/speckit.tasks` to create `tasks.md`.
