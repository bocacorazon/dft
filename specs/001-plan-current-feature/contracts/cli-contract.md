# CLI Contract: Planning Workflow

## Scope
Contract for generating and validating planning artifacts on the current feature branch.

## Command
`/speckit.plan` (or equivalent agent execution following `.specify/scripts/bash/setup-plan.sh --json`)

## Inputs
- Active git branch must be feature-formatted (e.g., `001-plan-current-feature`)
- Existing feature spec path from setup output (`FEATURE_SPEC`)
- Plan path from setup output (`IMPL_PLAN`)

## Required Outputs
- `specs/<branch>/plan.md`
- `specs/<branch>/research.md`
- `specs/<branch>/data-model.md`
- `specs/<branch>/quickstart.md`
- `specs/<branch>/contracts/*`
- Updated plan reference in `.github/copilot-instructions.md` markers

## Behavioral Rules
- Surface executable `before_plan` and `after_plan` hooks from `.specify/extensions.yml`
- Ignore hooks with `enabled: false`
- Skip hooks with non-empty `condition` (condition handled by HookExecutor)
- Fail on unresolved clarifications or unjustified constitution gate violations

## Exit Criteria
- Successful run when all required outputs exist and branch/path report is emitted
- Failure when setup fails, required artifacts are missing, or unresolved gate blockers remain
