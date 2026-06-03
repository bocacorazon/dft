# dft User Manual

`dft` is a headless workflow engine for spec-driven software production. It
splits work into four contract-driven phases — Intent, Solution, Build, and
Evaluate — each with specialized agents and deterministic tooling. You drive
it from inside Hermes (the coding agent) or via the CLI.

## The four phases

```
┌──────────────────────────────────────────────────────────┐
│ PHASE 1: INTENT     (Hermes agents)                      │
│ Raw demand → DemandPackage.json                          │
│ Agents: ambiguity-scanner → demand-refiner → ac-verifier │
├──────────────────────────────────────────────────────────┤
│ PHASE 2: SOLUTION   (Hermes agents)                      │
│ DemandPackage → SolutionDesign.json                      │
│ Agents: test-planner → wbs-author → surface-contract     │
│         → lane-assignment                                │
├──────────────────────────────────────────────────────────┤
│ PHASE 3: BUILD       (dft build CLI)                     │
│ SolutionDesign → Dispatched specs → Code on increment    │
│ Executors: speckit, direct, stub                         │
├──────────────────────────────────────────────────────────┤
│ PHASE 4: EVALUATE    (dft evaluate CLI)                  │
│ Code + TestPlan + Eval surfaces → Verdict                │
│ Engine: readiness → BDD eval → verdict                   │
└──────────────────────────────────────────────────────────┘
```

Each phase consumes a well-defined JSON contract from the previous phase and
produces a contract for the next. A coding agent (Hermes) can drive them
end-to-end, or you can trigger each phase individually.

## What dft manages

| Term | Meaning |
| --- | --- |
| Run | One execution attempt, identified by a run ID. |
| Demand package | The normalized JSON version of the user's request (`demand-package.json`). |
| Solution design | The test plan, WBS, lane assignments, and eval surfaces (`solution-design.json`). |
| Spec | One independently executable unit of work from the WBS. |
| Lane | The execution strategy assigned to a spec (`speckit`, `direct`, `stub`). |
| Increment branch | The integration branch for one run, named `increment/<run-id>`. |
| Run artifacts | Durable outputs under `.dft/runs/<run-id>/`. |

## Prerequisites

- A Git repository with an initial commit and a known default branch.
- Go if you are building `dft` from source.
- Hermes (the coding agent) for phases 1-2. For CLI-only workflows, you can
  author the contract JSON files manually.
- For real agent-backed build runs: GitHub Copilot CLI available as `copilot`
  (or passed with `--copilot-binary`).
- For smoke tests: no agent account is required; use the `stub` executor.

## Quickstart (Hermes-driven)

This is the primary workflow. Do everything from inside Hermes without
leaving the conversation.

### 1. Build the CLI

```sh
go build -o bin/dft ./cmd/dft
```

### 2. Provision dft into the repository once

```sh
./bin/dft init
git add .
git commit -m "provision dft assets"
```

### 3. Design your feature (Intent phase)

In Hermes:

```
> Help me design a REST API for managing bookmarks. Should support CRUD,
  tagging, and search. Keep it simple — single user, no auth for MVP.
```

Hermes loads the intent agents:
- **ambiguity-scanner** finds unclear terms: *"fast", "tagging model", "search scope"*
- You clarify in conversation
- **demand-refiner** produces a refined demand with acceptance criteria
- **ac-verifier** checks coverage: *"7/7 requirements covered. Verified."*
- You say **"Save the demand package"** → `demand-package.json` written

### 4. Design the solution (Solution phase)

In Hermes:

```
> Now design the solution
```

Hermes loads the solution agents:
- **test-planner** → 12 Gherkin scenarios covering all ACs
- **wbs-author** → 4 specs (bookmark CRUD, tagging, search, CLI surface)
- **surface-contract-author** → 2 eval surfaces (HTTP API, CLI)
- **lane-assignment** → 3 speckit lanes, 1 direct lane
- You say **"Save it"** → `solution-design.json` written

### 5. Build it

In Hermes:

```
> Build it
```

Hermes runs `dft build <run-id>`. The dispatcher reads the solution design,
creates the increment branch, and executes each spec via its assigned lane:

```
spec/bookmark-crud     → speckit  → specify → plan → tasks → implement → review → mergeback
spec/tagging           → speckit  → (same pipeline)
spec/search            → speckit  → (same pipeline)
spec/cli-surface       → direct   → single implement pass
```

```
4/4 specs complete
```

### 6. Evaluate it

In Hermes:

```
> Evaluate it
```

Hermes runs `dft evaluate <run-id>`. The eval engine:
1. Binds eval surfaces to delivered artifacts
2. Runs readiness probes
3. Authors a BDD eval plan (source-blind)
4. Executes scenarios against the increment
5. Reports verdict and coverage

```
verdict=pass coverage=12/12
```

### 7. Monitor progress

```
> dft status
```

Shows phase progress for all runs:
```
run-20260601-195500  intent=complete  solution=complete  build=4/4  eval=complete
run-20260601-120000  intent=complete  solution=incomplete  build=-  eval=-
```

```
> dft inspect run-20260601-195500
```

Shows full phase details, contract artifacts, per-spec status, and eval verdict.

---

## CLI reference

The dft CLI is the engine that Hermes calls under the hood. You can also use
it directly for scripting or when not using Hermes.

### Commands

| Command | Usage | What it does |
| --- | --- | --- |
| `help` | `dft help` | Prints the top-level help text. |
| `init` | `dft init [--force]` | Provisions managed `.dft/`, `.github/agents/`, `.specify/`, and related assets. |
| `sync` | `dft sync [--force]` | Refreshes managed assets using the provisioning manifest. |
| `build` | `dft build <run-id> [--spec <id>] [--resume] [--adapter stub\|copilot] [--agent-timeout 30m]` | Dispatches specs from `solution-design.json` to executors. |
| `build status` | `dft build status <run-id>` | Shows per-spec execution status. |
| `evaluate` | `dft evaluate <run-id>` | Runs readiness → BDD eval → verdict from contracts. |
| `evaluate inspect` | `dft evaluate inspect <run-id>` | Shows the last evaluation result. |
| `status` | `dft status` | Lists all runs with phase progress (intent, solution, build, eval). |
| `inspect` | `dft inspect <run-id>` | Full run inspection: artifacts, contracts, per-spec status, eval results. |
| `cancel` | `dft cancel <run-id>` | Marks a run cancelled. Artifacts stay on disk. |
| `resume` | `dft resume <run-id>` | Resumes a build from the last incomplete spec. |
| `submit` | `dft submit [flags] <demand>` | **Deprecated.** Creates a run from raw demand. The `--full`/`--dogfood` flags still work but print a deprecation notice. Use the phase commands instead. |

### `build` flags

| Flag | Meaning |
| --- | --- |
| `--adapter stub\|copilot` | Selects the agent adapter. Default is `stub` (smoke tests). Use `copilot` for real agent-backed runs. |
| `--copilot-binary <path>` | Overrides the Copilot executable. |
| `--agent-timeout <duration>` | Per-agent timeout, e.g. `30m` or `45m`. |
| `--spec <id>` | Execute only a single spec (skips others). |
| `--resume` | Resume from the last incomplete spec in a prior run. |

---

## CLI-driven workflow (without Hermes)

If you prefer to work without Hermes, you can author the contract JSON files
directly and use the CLI for phases 3-4.

### 1. Create the demand package

Write `.dft/runs/<run-id>/intent/demand-package.json`:

```json
{
  "id": "feature-001",
  "title": "Bookmark REST API",
  "raw_demand": "Build a REST API for managing bookmarks with CRUD, tagging, and search",
  "refined_demand": "Build a REST API...",
  "acceptance_criteria": [
    {"id": "AC-001", "description": "POST /bookmarks creates a bookmark and returns 201"},
    {"id": "AC-002", "description": "GET /bookmarks returns all bookmarks as JSON array"}
  ],
  "assumptions": ["Single-user deployment", "SQLite backend"],
  "non_goals": ["Authentication", "Sharing", "Import/export"],
  "verified_complete": true,
  "created_at": "2026-06-01T19:55:00Z"
}
```

### 2. Create the solution design

Write `.dft/runs/<run-id>/design/solution-design.json`:

```json
{
  "demand_package_id": "feature-001",
  "test_plan": {
    "demand_package_id": "feature-001",
    "scenarios": [
      {
        "id": "SC-001", "name": "Create bookmark",
        "requirement_ids": ["AC-001"],
        "given": ["the API is running"],
        "when": ["POST /bookmarks with valid JSON body"],
        "then": ["status is 201", "response contains id field"]
      }
    ]
  },
  "wbs": {
    "demand_package_id": "feature-001",
    "specs": [
      {"id": "spec-crud", "description": "CRUD endpoints", "acceptance_criteria": ["AC-001", "AC-002"]},
      {"id": "spec-search", "description": "Search endpoint", "acceptance_criteria": ["AC-003"]}
    ]
  },
  "lane_assignments": [
    {"spec_id": "spec-crud", "lane": "speckit", "rationale": "Multi-file feature"},
    {"spec_id": "spec-search", "lane": "speckit", "rationale": "Multi-file feature"}
  ],
  "eval_surface_contract": {
    "demand_package_id": "feature-001",
    "surfaces": [
      {"id": "http-api", "kind": "http_api", "artifact_ref": "http://localhost:8080", "adapter_family": "http", "environment_class": "ephemeral"}
    ]
  }
}
```

### 3. Build and evaluate

```sh
./bin/dft build feature-001 --adapter copilot --copilot-binary copilot
./bin/dft evaluate feature-001
```

---

## Reading command output

### `dft status`

```
run-20260601-195500  intent=complete  solution=complete  build=4/4  eval=complete
 spec/001-bookmark-crud  speckit  completed
 spec/002-tagging        speckit  completed
 spec/003-search         speckit  completed
 spec/004-cli-surface    direct   completed
run-20260601-120000  intent=complete  solution=incomplete  build=-  eval=-
```

Phase progress columns: `-` (not started), `incomplete` (prior phase done but this one missing), `N/M` (build progress with spec counts), `complete`.

### `dft inspect <run-id>`

First prints the artifact file tree under `.dft/runs/<run-id>/`, then:

```
run: run-20260601-195500
  intent:    complete
  solution:  complete
  build:     4/4
  evaluate:  complete

--- Demand Package ---
  title: Bookmark REST API
  acs: 7
  verified: true

--- Solution Design ---
  specs: 4
  scenarios: 12
  surfaces: 2
  lane: spec-crud -> speckit (Multi-file feature)
  lane: spec-search -> speckit (Multi-file feature)

--- Build Results ---
  spec-crud: completed (speckit)
  spec-search: completed (speckit)

--- Evaluation ---
{"verdict":"pass","coverage":{"total":12,"covered":12},"findings":[]}
```

---

## Key files and directories

| Path | Contents |
| --- | --- |
| `.dft/state.db` | Durable run state. |
| `.dft/skills/` | Hermes agent skill documents for intent and solution phases. |
| `.dft/runs/<run-id>/intent/demand-package.json` | Intent phase output — the verified demand package. |
| `.dft/runs/<run-id>/design/solution-design.json` | Solution phase output — test plan, WBS, lanes, eval surfaces. |
| `.dft/runs/<run-id>/design/wbs.json` | Work breakdown structure (individual artifact from wbs-author). |
| `.dft/runs/<run-id>/design/test-plan.json` | Gherkin test scenarios (individual artifact from test-planner). |
| `.dft/runs/<run-id>/design/lane-assignments.json` | Spec-to-lane mapping (individual artifact from lane-assignment). |
| `.dft/runs/<run-id>/design/eval-surfaces.json` | Eval surface declarations. |
| `.dft/runs/<run-id>/orchestration-result.json` | Build phase output — per-spec results and artifact manifest. |
| `.dft/runs/<run-id>/eval/evaluation.json` | Evaluation phase output — verdict, coverage, findings, evidence. |
| `.dft/runs/<run-id>/steps/` | Per-step transcripts, parsed output, stderr/stdout from agent runs. |
| `.dft/worktrees/<run-id>/<spec-id>/` | Per-spec worktree and generated Spec Kit artifacts. |

---

## Executors (execution plans)

Each spec in the WBS is assigned a lane that maps to an executor:

| Lane | Executor | When to use |
| --- | --- | --- |
| `speckit` | Full specify → plan → tasks → analyze → implement → code-review → mergeback pipeline. | Multi-file features, complex logic, new modules. |
| `direct` | Single-pass implementation without spec/plan overhead. | Config changes, single-file scripts, dependency updates. |
| `stub` | No-op that returns "completed" immediately. | Pipeline smoke tests, dry runs. |

New executors can be registered via the `ExecutorRegistry` in `internal/orchestration/v2/executor.go`.

---

## Working on dft itself

When you are changing `dft`, the repository constitution expects test-first Go
development. Keep `go test ./...` green, use `go vet ./...` as the additional
static check, and rebuild the CLI with `go build ./cmd/dft` before manual smoke
or end-to-end runs.

For a real GitHub/Copilot-backed operator check, use `scripts/real-e2e.sh`.
