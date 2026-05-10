# Dark Factory Toolkit Foundation Design

This document summarizes the current foundation shape. The fuller option analysis lives in [docs/foundation/design-discussion.md](design-discussion.md), and the staged implementation plan lives in [docs/foundation/implementation-plan.md](implementation-plan.md).

## Purpose

Dark Factory Toolkit (DFT) is a spec-driven development harness for turning intent into validated software artifacts. It coordinates agents, tools, functions, and workflows through explicit artifacts and deterministic flows rather than relying on an autonomous chat session to improvise the development process.

The initial implementation target is a local execution engine. Future execution backends can package the same flow model for Docker, Kubernetes, GitHub Actions, or other runners.

DFT uses an agency gradient. Intent development is highly agentic and conversational. Solution Design is agent-assisted and collaborative. Orchestration is automated: once the intent and design artifacts are cemented, the Dark Factory runs without ongoing human steering and reports its result through evidence and verdicts.

## Design Principles

- Put agency where ambiguity is highest, then reduce it as work becomes executable.
- Orchestration is deterministic: the engine parses and executes a rigid DAG.
- Agents produce artifacts: agents draft, validate, design, convert, and critique, but they do not choose the runtime control flow.
- Interfaces are typed: each phase emits structured artifacts that downstream phases can validate.
- Evaluation starts early: the Eval thread generates its strategy at the beginning of orchestration, then validates the Build output when artifacts are ready.
- Models are configurable: model selection is resolved by policy and can be overridden by orchestrator, lane, flow, or step.
- Local-first: the first engine should be inspectable, resumable, and easy to debug on a developer workstation.

## Phase Model

DFT is organized into three major phases.

### 1. Intent

The Intent phase converts a user goal into a strict Demand Package.

The workflow starts conversationally. An intent agent conducts a session with the user and drafts a human-readable description of the desired project, feature, increment, or fix. Validation agents then review the draft for completeness, ambiguity, conflicts, missing constraints, and acceptance criteria quality. Once the draft is complete enough, a generation agent emits a strict YAML or JSON Demand Package.

The Demand Package is the source of truth for downstream phases. It contains the artifact description, scope, constraints, assumptions, risks, non-goals, and acceptance criteria.

Primary artifact:

- [docs/foundation/schemas/demand-package.schema.yaml](schemas/demand-package.schema.yaml)

### 2. Solution Design

The Solution Design phase converts a Demand Package into executable planning artifacts.

It is intentionally more than a WBS generator. It should include these agents:

- Architecture Design Agent: creates architecture blueprints, boundaries, components, integration points, and technical decisions.
- Eval Criteria Conversion Agent: converts acceptance criteria into machine-usable evaluation inputs, strategies, evidence requirements, and verdict rules.
- Lane Selector Agent: evaluates complexity and assigns the work to a lane such as `streamlined`, `spec`, or `recursive`.
- WBS Generator: produces a human-readable work breakdown and a machine-readable WBS for the orchestrator.

The WBS remains conceptual. It describes the work and maps each task to predefined Flow Templates. The WBS does not generate arbitrary new control flow at runtime.

Primary artifacts:

- [docs/foundation/schemas/wbs.schema.yaml](schemas/wbs.schema.yaml)
- [docs/foundation/schemas/eval-plan.schema.yaml](schemas/eval-plan.schema.yaml)

### 3. Orchestration

The Orchestration phase executes the selected lane and flow templates.

When a WBS is submitted, the orchestrator expands WBS tasks into predefined Flow Templates and starts two parallel threads:

- Build thread: executes the flow DAG and produces implementation artifacts.
- Eval thread: generates an adversarial evaluation plan at the beginning of orchestration, waits for the Build thread to finish, then evaluates the produced artifacts and emits a verdict.

The Eval thread is autonomous in strategy generation, but it is not allowed to change Build execution order. It can only observe inputs, generate tests or review strategies, and validate final artifacts.

Primary artifact:

- [docs/foundation/schemas/flow-template.schema.yaml](schemas/flow-template.schema.yaml)

## Local Engine Architecture

The initial local engine should be split into small components with explicit responsibilities.

### Artifact Store

Stores Demand Packages, architecture blueprints, eval plans, WBS documents, flow runs, logs, captured outputs, and verdicts. The first version can use filesystem-backed run directories.

### Registry

Resolves named lanes, flow templates, agents, tools, functions, workflows, and model policies. The registry is the boundary between structured plans and executable implementations.

### Model Resolver

Selects the model for each agent invocation. Resolution order should be:

1. step override
2. flow override
3. lane default
4. orchestrator run override
5. global default

### Scheduler

Parses flow templates, validates that they form an acyclic graph, resolves dependencies, and dispatches runnable steps. The first local scheduler can use a thread pool with a conservative concurrency limit.

### Runner Interface

Executes one unit of work. A runner handles one of these types:

- `agent`: invokes an LLM or coding agent through a configured adapter.
- `tool`: invokes a registered external tool or CLI.
- `function`: invokes a local deterministic function.
- `workflow`: invokes another registered flow.

### Verification Engine

Runs setup checks, step-level verification, retry policies, and final run verification. It should collect structured evidence rather than only pass/fail text.

### Event Log

Records every state transition, input artifact reference, output artifact reference, model choice, command, verification result, retry, and failure. This makes local runs replayable and debuggable.

## Orchestration Lifecycle

1. Validate the Demand Package.
2. Run Solution Design agents.
3. Produce architecture blueprint, eval plan, lane selection, and WBS.
4. Validate the WBS against available Flow Templates.
5. Resolve model policies and runtime settings.
6. Start Build and Eval threads.
7. Eval thread generates the adversarial evaluation suite from the Demand Package, architecture blueprint, and WBS.
8. Build thread executes the rigid flow DAG.
9. Orchestrator freezes Build artifacts for evaluation.
10. Eval thread executes the evaluation suite and emits a verdict.
11. Orchestrator writes a final run report with evidence, failures, and suggested remediation.

## Lane Model

Lanes provide a workflow envelope for different complexity levels.

- `streamlined`: narrow changes, small bug fixes, low ambiguity, minimal architecture work.
- `spec`: normal feature work using the full spec-driven path.
- `recursive`: large or uncertain efforts that should be decomposed into nested Demand Packages or subprojects.

Lane selection should be agent-assisted and evidence-based. The selector should produce both the selected lane and a rationale with complexity signals such as scope size, ambiguity, number of affected systems, safety risk, novelty, and expected verification depth.

## Flow Template Model

Flow Templates are reusable DAG definitions. A WBS task binds concrete inputs to one of these templates.

The current draft in [docs/flow-dsl.yaml](../flow-dsl.yaml) should evolve from a list-shaped sketch into a validated graph format with:

- stable step IDs
- explicit `depends_on` relationships
- typed step runners
- input and output bindings
- setup hooks
- verification hooks
- retry and failure policies
- capture controls
- model override fields

The engine must reject invalid flows before execution.

## Eval Model

The Eval thread should behave like an adversarial reviewer, not a passive checklist runner.

Evaluation strategies may include:

- acceptance-criteria traceability checks
- generated tests
- static analysis
- code review prompts
- artifact diff review
- API or UI behavior checks
- regression checks
- security and reliability probes where relevant

Each evaluation must produce structured evidence and map back to one or more acceptance criteria. The final verdict should distinguish between pass, fail, inconclusive, and blocked.

## Agent Harness Inspiration

The design discussion should evaluate whether DFT should use a custom local runner, an existing graph-oriented agent framework, or a general workflow engine as its substrate. The current foundation direction is to keep the core artifact and flow model independent from any single harness until that comparison is made.

- Graph-oriented agent frameworks are useful inspiration for explicit state, nodes, edges, checkpoints, and resumability.
- Role-based multi-agent frameworks are useful inspiration for the Intent and Solution Design phases, but they are a weaker fit for the runtime core because DFT wants rigid orchestration.
- Workflow engines such as Temporal, Prefect, Dagster, and Airflow are useful inspiration for durability, retries, observability, dependency scheduling, and run history.
- Coding-agent harnesses are useful inspiration for sandboxed workspaces, command execution, tool registries, transcript capture, and patch review.
- Spec-driven systems are useful inspiration for the artifact ladder from intent to plan to tasks to implementation to validation.

OpenClaw and Hermes should be evaluated as concrete references once their exact target projects and versions are pinned down. The foundation architecture should stay compatible with their strongest likely patterns: typed tool registries, workspace isolation, explicit run logs, model abstraction, and repeatable agent invocations.

## Initial Implementation Slices

The detailed staged plan is maintained in [docs/foundation/implementation-plan.md](implementation-plan.md). The high-level sequence is:

1. Define schemas for the core artifacts.
2. Build a filesystem-backed artifact store.
3. Build a flow template validator.
4. Build a local DAG scheduler for `function` and `tool` steps.
5. Add `agent` runner adapters.
6. Add the Solution Design pipeline.
7. Add the parallel Build/Eval orchestration lifecycle.
8. Add run reports and verdict evidence.

The first runnable milestone should avoid complex coding-agent behavior. It should prove that a WBS can bind to a Flow Template, the local engine can execute a deterministic DAG, and the run report can connect outputs to acceptance criteria.