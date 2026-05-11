# Data Model: 001-plan-current-feature

## Entity: FeaturePlan
- **Purpose**: Represents the planning record for the current feature branch.
- **Fields**:
  - `branch` (string, required, format: `NNN-short-name`)
  - `spec_path` (string, required)
  - `plan_path` (string, required)
  - `status` (enum: `draft|complete`)
  - `created_at` (date/time, required)

## Entity: ResearchDecision
- **Purpose**: Captures each resolved planning unknown.
- **Fields**:
  - `id` (string, required)
  - `topic` (string, required)
  - `decision` (string, required)
  - `rationale` (string, required)
  - `alternatives` (string list, required)
  - `source_artifact` (string, required; e.g., `research.md`)

## Entity: Artifact
- **Purpose**: Tracks outputs expected from `/speckit.plan`.
- **Fields**:
  - `name` (enum: `plan.md|research.md|data-model.md|quickstart.md|contracts/*`)
  - `path` (string, required)
  - `phase` (enum: `phase0|phase1`)
  - `exists` (boolean, required)

## Entity: Hook
- **Purpose**: Models plan extension hook metadata from `.specify/extensions.yml`.
- **Fields**:
  - `timing` (enum: `before_plan|after_plan`)
  - `extension` (string, required)
  - `command` (string, required)
  - `optional` (boolean, required)
  - `enabled` (boolean, default: true)
  - `condition` (string|null)

## Relationships
- A `FeaturePlan` has many `ResearchDecision` records.
- A `FeaturePlan` has many `Artifact` records.
- A `FeaturePlan` references zero or more `Hook` entries for pre/post execution messaging.

## Validation Rules
- `branch` must match current git branch.
- All required `Artifact.exists` values must be `true` before marking `FeaturePlan.status = complete`.
- Hooks with non-empty `condition` are not executed by this plan artifact phase.

## State Transitions
- `FeaturePlan.status`: `draft -> complete`
  - Transition requires: pre-hooks evaluated, plan artifacts generated, post-hooks surfaced.
