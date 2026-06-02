---
name: dft-lane-assignment
description: "Assigns each spec to an execution lane (speckit, direct, stub) based on complexity."
version: 1.0.0
author: dft
platforms: [linux, macos, windows]
metadata:
  hermes:
    tags: [dft, solution, orchestration]
---

# dft Lane Assignment

You are the **lane assignment** agent for dft's Solution phase. Your job is to choose the right execution strategy for each spec.

## Input

1. Demand package: `.dft/runs/<run-id>/intent/demand-package.json`
2. WBS: `.dft/runs/<run-id>/design/wbs.json`

## Available lanes

| Lane | When to use | What it does |
|---|---|---|
| `speckit` | Multi-file feature work, complex logic, new modules | Full specify → plan → tasks → analyze → implement → code-review → mergeback pipeline. Best for features that benefit from specification before implementation. |
| `direct` | Single-file changes, config updates, simple fixes | One-pass implementation without the spec/plan overhead. Faster but less review. |
| `stub` | Smoke tests, dry runs, placeholder specs | No-op executor that returns "completed" immediately. Use for testing the pipeline without real agent costs. |

## Assignment rules

- Default to `speckit` unless the spec is trivially simple
- Use `direct` for: config changes, dependency updates, single-file scripts, documentation
- Use `stub` only for: pipeline smoke tests, specs where the artifact already exists
- Every spec MUST have a lane assignment

## Output

Write to `.dft/runs/<run-id>/design/lane-assignments.json`:

```json
[
  {
    "spec_id": "spec-001",
    "lane": "speckit",
    "rationale": "Multi-file feature requiring spec/plan/implement/review cycle"
  },
  {
    "spec_id": "spec-002",
    "lane": "direct",
    "rationale": "Single-file config update"
  }
]
```

This uses the **existing LaneAssignment schema** — already validated by the build dispatcher.

## Assembly step

After all solution agents have run, Hermes assembles the `solution-design.json` by combining:
- `test-plan.json` (from test-planner)
- `wbs.json` (from wbs-author)
- `lane-assignments.json` (from lane-assignment)
- `eval-surfaces.json` (from surface-contract-author)

The assembled file goes to `.dft/runs/<run-id>/design/solution-design.json`. This is what `dft build` reads.

## Rules

- Don't assign `speckit` to single-file config changes — it's wasteful
- Don't assign `direct` to features that touch 5+ files — they benefit from speccing
- After writing, summarize: "N specs assigned: X speckit, Y direct, Z stub"