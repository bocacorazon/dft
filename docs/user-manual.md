# dft User Manual

`dft` is the **execution layer** for a frozen software-delivery flow. It does
not require that design artifacts come from Hermes, from in-repo agents, or
from manual authoring. It consumes explicit inputs and executes them
deterministically.

For upstream design material, see [design-phase/README.md](design-phase/README.md).

## Primary commands

`dft` now has two primary execution surfaces:

| Command | Purpose |
| --- | --- |
| `dft build` | Execute **one frozen spec flow** from explicit runtime inputs. |
| `dft submit` | Execute a **WBS DAG** by dispatching one frozen spec flow per ready spec. |

## Submission inputs

The `submit` command is defined by the inputs it consumes, **not** by how those
inputs were created.

| Input | Required | Meaning |
| --- | --- | --- |
| WBS JSON path | Yes | The DAG of specs to execute. The WBS may also carry the top-level branch envelope (`base_branch`, `increment_branch`) plus per-spec execution metadata such as dependencies, workflow override, prompt path, feature slug, and model family. |
| Workflow name or flow path | Yes | The flow used to execute each spec unless a spec overrides it. |
| Base branch | No | Optional override for the WBS `base_branch`. If omitted, dft falls back to the WBS value and then the repo default branch. |
| Increment branch | No | Optional override for the WBS `increment_branch`. Specs clone from and merge back into this branch. If omitted, dft falls back to the WBS value and then `base_branch`. |
| Model config path | Yes | A config file mapping a model family to `xhigh`, `high`, `medium`, and `low` concrete model IDs. |
| Model family | No | The family to resolve workflow `model_type` tiers against. If omitted, dft uses the config default when available. |
| Callback URL | No | An HTTP endpoint that receives spec-completion and run-completion notifications. |
| Repository with dft assets | Yes | The target repository where execution runs, flows, and audit artifacts live. |

Those inputs may come from:

- Hermes or another design-agent environment
- hand-authored JSON/YAML/files
- another orchestration service
- a CI system or automation wrapper

`dft submit` treats them the same way once they exist as files and flags.

## Single-spec execution inputs

`dft build` executes one spec flow directly.

| Input | Required | Meaning |
| --- | --- | --- |
| Prompt text or prompt file | Yes | The frozen implementation prompt for the spec. |
| Workflow name or flow path | Yes | The exact flow to execute. |
| Feature slug | Yes | The spec/work identifier for the execution. |
| Increment branch | No | The branch the spec execution clones from and merges back into. If omitted, dft resolves the repo default branch. |
| Model config path | Yes | The config used to resolve workflow model tiers. |
| Model family | No | Explicit family override for model resolution. |

## Quickstart

Provision assets once:

```sh
dft init
```

Execute one frozen spec:

```sh
dft build \
  --prompt-file specs/001-auth/prompt.md \
  --workflow single-spec \
  --feature-slug 001-auth \
  --increment-branch increment/auth-platform \
  --models config/models.json
```

Execute a WBS submission:

```sh
dft submit \
  --wbs design/wbs.json \
  --workflow single-spec \
  --models config/models.json \
  --callback-url https://example.com/dft/callback
```

Inspect results:

```sh
dft status
dft inspect <run-id>
```

## CLI reference

### Core commands

| Command | Usage | What it does |
| --- | --- | --- |
| `help` | `dft help` | Prints the top-level help text. |
| `init` | `dft init [--force]` | Provisions managed `.dft/`, `.github/agents/`, `.specify/`, and related assets. |
| `sync` | `dft sync [--force]` | Refreshes managed assets using the provisioning manifest. |
| `build` | `dft build --prompt <text>\|--prompt-file <path> --workflow <name>\|--flow <path> --feature-slug <slug> --models <path> [--increment-branch <branch>] [--model-family <family>] [--adapter stub\|copilot]` | Executes one frozen spec flow. |
| `submit` | `dft submit --wbs <path> --workflow <name>\|--flow <path> --models <path> [--base-branch <branch>] [--increment-branch <branch>] [--model-family <family>] [--callback-url <url>] [--adapter stub\|copilot]` | Executes a WBS DAG by dispatching frozen spec flows. |
| `evaluate` | `dft evaluate <run-id>` | Runs readiness → BDD eval → verdict from evaluation contracts. |
| `status` | `dft status` | Lists runs and progress. |
| `inspect` | `dft inspect <run-id>` | Shows run artifacts and execution details. |
| `cancel` | `dft cancel <run-id>` | Marks a run cancelled. Artifacts stay on disk. |
| `resume` | `dft resume <run-id>` | Resumes a supported interrupted run. |

### Compatibility note

Legacy `submit`/`build` shapes still exist as transitional compatibility paths
for the older design-owned pipeline. They are not the preferred contract and are
documented as migration surfaces rather than the primary product posture.

## Audit artifacts

Every run writes durable artifacts under `.dft/runs/<run-id>/`, including:

- rendered step inputs
- stdout/stderr and structured parsed output
- execution and orchestration summaries
- callback/audit side effects

This is the stable operational contract for dft as an execution layer.
