# Dark Factory Toolkit — Execution Process

This document describes the **execution-layer** behavior of dft.

It focuses on what `dft submit` and `dft build` consume and what they produce.
It intentionally treats upstream design as an external concern. dft only cares
that the required execution artifacts exist and conform to the expected shape.

For upstream design docs, see [../design-phase/README.md](../design-phase/README.md).

## Primary execution inputs

### `dft submit`

| Input | Description |
| --- | --- |
| Repository | The target Git repository where execution happens. |
| dft assets | Provisioned flows, agents, context, and state files in the repository. |
| WBS JSON | The DAG of specs to execute. |
| Optional base branch | Overrides or supplies the WBS `base_branch`; defaults to the repo default branch when absent. |
| Optional increment branch | Overrides or supplies the WBS `increment_branch`; specs branch from and merge back into it. |
| Workflow name or flow path | The execution flow used for each spec unless overridden per spec. |
| Model config | A file mapping model families to `xhigh`, `high`, `medium`, and `low`. |
| Optional model family | The family to use for workflow model-tier resolution. |
| Optional callback URL | A completion-report endpoint for spec/run notifications. |

### `dft build`

| Input | Description |
| --- | --- |
| Repository | The target Git repository where execution happens. |
| Prompt text or prompt file | The frozen implementation prompt for one spec. |
| Workflow name or flow path | The execution flow for the spec. |
| Feature slug | The spec/work identifier for the run. |
| Optional increment branch | The branch the spec execution clones from and merges back into; defaults to the repo default branch. |
| Model config | A file mapping model families to `xhigh`, `high`, `medium`, and `low`. |
| Optional model family | The family to use for workflow model-tier resolution. |

These inputs may come from Hermes, another orchestration service, CI, or manual
authoring. dft does not distinguish among those sources.

## `dft submit` process

| Step | Inputs | Outputs |
| --- | --- | --- |
| 1. Request validation | WBS path, workflow reference, branch overrides, model config path | A validated orchestration request. |
| 2. WBS loading | WBS JSON | Parsed WBS with spec list, dependency graph, and branch envelope. |
| 3. Branch resolution | WBS branch envelope, optional overrides, repository default branch | Resolved `base_branch` and `increment_branch` for the run. |
| 4. Dependency scheduling | WBS DAG and prior spec results | The set of ready specs eligible for execution. |
| 5. Single-spec dispatch | One ready spec plus shared workflow/model/branch inputs | One per-spec execution run. |
| 6. Spec completion reporting | Spec result and optional callback URL | Callback payload for `spec.completed` when configured. |
| 7. Run completion reporting | Aggregate run result and optional callback URL | Callback payload for `run.completed` when configured. |
| 8. Run summary persistence | All spec results and resolved branch topology | `execution-orchestration.json` under `.dft/runs/<run-id>/`. |

## `dft build` process

| Step | Inputs | Outputs |
| --- | --- | --- |
| 1. Request validation | Prompt source, workflow reference, feature slug, increment branch, model config | A validated execution request. |
| 2. Prompt loading | Prompt text or prompt file | The rendered execution prompt. |
| 3. Workflow loading | Workflow name or flow path | A parsed flow definition. |
| 4. Branch resolution | Explicit `increment_branch` or repository default branch | The branch context bound into the flow run. |
| 5. Model resolution | Workflow `model_type` entries plus model config | A runnable flow with concrete model IDs. |
| 6. Flow execution | Prompt, workflow, feature slug, branch context, repository context | Step artifacts under `.dft/runs/<run-id>/steps/`. |
| 7. Result persistence | Final execution result plus resolved branch metadata | `execution-result.json` under `.dft/runs/<run-id>/`. |

## WBS expectations

The WBS is the orchestration contract. At minimum it must provide:

- `demand_package_id`
- optional `base_branch`
- optional `increment_branch`
- `specs[]`
- for each spec: `id`, description or `prompt_path`, and acceptance criteria

The current execution path also supports per-spec metadata such as:

- `depends_on`
- `workflow` or `workflow_path`
- `feature_slug`
- `model_family`

## Outputs

The main outputs of the execution layer are:

| Output | Meaning |
| --- | --- |
| `.dft/runs/<run-id>/execution-result.json` | Single-spec execution summary. |
| `.dft/runs/<run-id>/execution-orchestration.json` | WBS orchestration summary. |
| `.dft/runs/<run-id>/steps/<step-id>/...` | Step-level audit artifacts. |
| callback payloads | Optional external completion notifications. |

## Relationship to design

dft no longer needs to own the design phase in order to be useful. The design
phase can live elsewhere as long as it emits the frozen inputs described above.

That separation is the core paradigm: **design produces artifacts; dft executes
artifacts**.
