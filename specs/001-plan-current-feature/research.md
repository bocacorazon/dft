# Research: 001-plan-current-feature

## Decision 1: Use repository evidence to fill Technical Context
- **Decision**: Use `go.mod`, existing scripts, and current repository layout as authoritative inputs for `plan.md`.
- **Rationale**: `spec.md` is still template-level, but project facts are concrete (Go 1.22, CLI workflow, bash tooling).
- **Alternatives considered**: Leave fields as `NEEDS CLARIFICATION` (rejected because available evidence is sufficient).

## Decision 2: Define contracts for CLI workflow, not HTTP APIs
- **Decision**: Create contracts in `contracts/` for CLI command behavior and flow I/O schemas.
- **Rationale**: This repository is a CLI/spec workflow project; contract surface is command invocation + structured outputs.
- **Alternatives considered**: OpenAPI endpoint contracts (rejected as mismatched to project type).

## Decision 3: Keep a minimal planning data model
- **Decision**: Model planning around `FeaturePlan`, `ResearchDecision`, `Artifact`, and `Hook`.
- **Rationale**: These entities cover branch-scoped planning outputs and extension-hook orchestration without over-modeling.
- **Alternatives considered**: Large runtime/orchestration model (rejected for this planning-only scope).

## Decision 4: Quickstart should focus on reproducible branch workflow
- **Decision**: Document setup-plan, artifact checks, tests, and follow-up commands for contributors.
- **Rationale**: The branch goal is planning artifact generation, so quickstart must optimize for repeatability and verification.
- **Alternatives considered**: Implementation-heavy quickstart (rejected because coding tasks are out of scope for `/speckit.plan`).
