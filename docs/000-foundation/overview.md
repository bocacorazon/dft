# Dark Factory Toolkit — Execution-Layer Overview

This document captures the execution-layer architecture of dft.

The core posture is:

> **design happens upstream; dft executes frozen artifacts**

For upstream design material, see [../design-phase/README.md](../design-phase/README.md).

## Purpose

`dft` is a headless workflow engine specialized for **executing one frozen spec
flow** and for **orchestrating many frozen specs through a WBS DAG**.

It is not required to generate the design artifacts it executes. Its job is to:

1. validate execution inputs
2. load flows and resolve model tiers
3. execute steps deterministically
4. record durable audit artifacts
5. orchestrate dependency-aware spec execution

## Core surfaces

| Surface | Purpose |
| --- | --- |
| `dft build` | Execute one frozen spec flow from explicit runtime inputs. |
| `dft submit` | Execute a WBS DAG by dispatching frozen spec flows. |
| `dft evaluate` | Run deterministic readiness/evaluation logic against produced artifacts. |

## Required contracts

### Single-spec execution

The execution contract for `dft build` is:

- prompt text or prompt file
- workflow name or flow path
- feature slug
- model config path
- optional model family override

### WBS orchestration

The orchestration contract for `dft submit` is:

- WBS JSON path
- workflow name or flow path
- model config path
- optional model family override
- optional callback URL

These contracts are source-agnostic. dft consumes them whether they came from
Hermes, another agent system, CI, or hand-authored files.

## Vocabulary

| Term | Meaning |
| --- | --- |
| Spec | One independently executable unit of work. |
| WBS | The DAG of specs to execute. |
| Flow | The YAML workflow that defines how one spec is executed. |
| Step | One command, agent, tool, gate, function, workflow, verify, or loop entry in a flow. |
| Model tier | An abstract workflow model selection (`xhigh`, `high`, `medium`, `low`). |
| Model config | A file that resolves model tiers to concrete provider model IDs. |
| Run | One execution attempt, identified by a run ID. |
| Audit artifacts | Durable step/run outputs written under `.dft/runs/<run-id>/`. |

## Architectural decisions

### 1. Execution-first boundary

dft should be excellent at running already-decided flows. It should not depend
on owning upstream design generation.

### 2. Two-layer orchestration

- **Inner layer:** execute one frozen spec flow.
- **Outer layer:** schedule specs from a WBS DAG and dispatch the inner layer.

### 3. Model-tier resolution

Flows should declare abstract capacity tiers, not hard-coded provider models.
Concrete model IDs are resolved at runtime from a model-family config.

### 4. Durable auditability

Execution inputs, outputs, parsed step results, and orchestration summaries are
recorded under `.dft/runs/<run-id>/`.

### 5. Compatibility during migration

Legacy design-owned paths and front-end agents remain present during the
migration, but they are compatibility surfaces rather than the primary product
shape.
