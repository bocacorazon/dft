# Submission Artifacts Consumed by dft

This document describes the **inputs dft consumes**, regardless of how they were
created.

The inputs may come from Hermes, another agent framework, CI automation, or
manual authoring. Once they exist as files and flags, dft treats them the same
way.

## 1. Single-spec execution artifacts

Used by `dft build`.

| Input | Required | Notes |
| --- | --- | --- |
| Prompt text or prompt file | Yes | The frozen implementation prompt. |
| Workflow name or flow path | Yes | The exact flow to run. |
| Feature slug | Yes | Used as the spec/work identifier. |
| Increment branch | No | The branch the spec execution clones from and merges back into. Defaults to the repo default branch when omitted. |
| Model config path | Yes | Resolves workflow `model_type` tiers. |
| Model family | No | Overrides the config default. |

## 2. WBS orchestration artifacts

Used by `dft submit`.

| Input | Required | Notes |
| --- | --- | --- |
| WBS JSON path | Yes | The DAG of specs to run. |
| Base branch override | No | Overrides or supplies the WBS `base_branch`. |
| Increment branch override | No | Overrides or supplies the WBS `increment_branch`. |
| Workflow name or flow path | Yes | Default workflow for specs unless overridden. |
| Model config path | Yes | Resolves workflow `model_type` tiers. |
| Model family | No | Default family for specs unless overridden. |
| Callback URL | No | Receives completion notifications. |

## 3. WBS expectations

At minimum, the WBS should provide:

- `demand_package_id`
- optional `base_branch`
- optional `increment_branch`
- `specs[]`
- for each spec:
  - `id`
  - description or `prompt_path`
  - acceptance criteria

The execution path also supports optional per-spec metadata:

- `depends_on`
- `workflow`
- `workflow_path`
- `feature_slug`
- `model_family`

## 4. Workflow expectations

The flow should be a YAML workflow definition understood by dft's flow loader.

When the flow uses models, prefer:

```yaml
model_type: low
```

instead of hard-coding a concrete provider model. dft resolves `model_type`
through the supplied model config.

## 5. Model config expectations

The model config should map a family to abstract tiers:

```yaml
default_family: openai
families:
  openai:
    xhigh: gpt-5.5
    high: gpt-5.5
    medium: gpt-5.3-codex
    low: gpt-5-mini
```

This lets the same workflow run against different model families without editing
the flow itself.
