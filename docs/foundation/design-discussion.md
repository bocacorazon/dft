# DFT Design Discussion

## Purpose

This document explores the design space for Dark Factory Toolkit before implementation choices are frozen. It captures options, tradeoffs, working recommendations, accepted discovery decisions, and open questions.

The goal is to avoid building a tool-shaped answer too early. DFT sits at the intersection of spec-driven development, agent harnesses, workflow engines, and evaluation systems. The foundation should make those tensions explicit.

## Core Product Thesis

DFT should follow an agency gradient:

- Intent is highly agentic and conversational. The user should have a powerful set of agents for discovering, sharpening, challenging, and cementing intention.
- Solution Design is assisted and collaborative. Agents help produce architecture blueprints, eval criteria, lane selection, and WBS artifacts, but the human can still shape the solution before it is committed.
- Orchestration is automated. Once intent and solution design are cemented, development proceeds as a Dark Factory run: deterministic, unattended, observable, and evaluated against the agreed artifacts.

This means DFT is not trying to maximize agent autonomy everywhere. It is trying to put agency where ambiguity is highest, then reduce agency as the work crosses into execution.

## Current Discovery Decisions

These decisions came from the initial discussion and should be treated as accepted unless reopened deliberately.

- The Demand Package should be strict structured data, produced after conversational drafting and validation.
- Runtime orchestration should be rigid rather than agent-routed.
- The WBS should map conceptual tasks to predefined Flow Templates.
- The Eval thread should generate its evaluation strategy at the beginning of orchestration, then execute after Build artifacts are ready.
- Lane selection should evaluate complexity rather than rely only on manual labels.
- The product boundary is: agent-rich Intent, agent-assisted Solution Design, automated Orchestration.

These are not implementation stack decisions. They describe desired product behavior.

## Design Axis 1: Center Of Gravity

### Option A: Artifact-First System

DFT is organized around typed artifacts: Demand Package, architecture blueprint, eval plan, WBS, flow template, run report, and verdict.

Strengths:

- Easier to audit, validate, diff, version, and resume.
- Better fit for spec-driven development.
- Makes Build/Eval traceability natural.
- Supports multiple execution backends later.

Weaknesses:

- More upfront schema and artifact design.
- Can feel heavier for small tasks.
- Requires strong artifact authoring ergonomics.

### Option B: Harness-First System

DFT is primarily a runner for tools, agents, and workflows. Artifacts exist, but the runtime abstraction is the product center.

Strengths:

- Faster path to executing useful flows.
- Better for experimenting with agent/tool composition.
- Easier to compare against agent harness products.

Weaknesses:

- Risk of under-specified inputs and outputs.
- Can become a generic workflow engine instead of a spec-driven toolkit.
- Evaluation traceability may become secondary.

### Option C: Conversation-First System

DFT treats chat sessions as the primary workspace. Structured artifacts are extracted from the conversation.

Strengths:

- Very natural for intent discovery.
- Preserves the Socratic feel the project wants.
- Good fit for early product exploration.

Weaknesses:

- Harder to automate reliably.
- Transcript state is difficult to validate and replay.
- More brittle for long-running orchestration.

### Working Recommendation

Use an artifact-first architecture with a strong conversational front door. Conversation is how humans shape intent; artifacts are how the system crosses phase boundaries.

## Design Axis 2: Where Agency Lives

### Option A: Agent-Managed Flow

A manager agent decides which step to run next.

Strengths:

- Flexible and adaptive.
- Can respond to unexpected discoveries.

Weaknesses:

- Hard to reproduce.
- Harder to test.
- Makes failure analysis murky.

### Option B: Engine-Managed Flow

The engine executes a validated graph. Agents only operate inside explicit nodes.

Strengths:

- Predictable execution.
- Easier retries, logs, and replay.
- Clear separation between control plane and agent output.

Weaknesses:

- Less adaptive at runtime.
- Requires better flow design up front.

### Option C: Negotiated Flow

The graph is deterministic, but selected decision nodes can request a human or agent decision from a bounded option set.

Strengths:

- Preserves determinism while allowing scoped adaptation.
- Useful for lane selection, remediation, and escalation.

Weaknesses:

- More complex state model.
- Requires careful boundaries to avoid becoming agent-routed flow.

### Working Recommendation

Use engine-managed flow for the orchestration core. Add negotiated decision nodes later if the deterministic model becomes too rigid.

## Design Axis 3: Flow Semantics

### Option A: Ordered Step List

The first DSL sketch is close to this: steps execute mostly in sequence.

Strengths:

- Simple to author.
- Simple to implement.

Weaknesses:

- Poor fit for parallelism.
- Dependencies are implicit.
- Harder to validate and visualize.

### Option B: DAG With Explicit Dependencies

Each step has a stable id and optional `depends_on` list.

Strengths:

- Good fit for local parallelism.
- Easy cycle detection.
- Clear scheduling model.
- Works across local, Docker, Kubernetes, and CI backends.

Weaknesses:

- Slightly more verbose authoring.
- Conditional flow requires additional modeling.

### Option C: State Machine

Steps can branch based on results and state transitions.

Strengths:

- More expressive.
- Better for remediation loops and human gates.

Weaknesses:

- More complex to validate and reason about.
- Can blur the line between orchestration and agent autonomy.

### Working Recommendation

Adopt explicit DAG semantics first. Treat conditional/state-machine behavior as a later extension once the artifact and event model is stable.

## Design Axis 4: WBS To Flow Mapping

### Option A: WBS Binds To Registered Flow Templates

The WBS describes conceptual work. Each task references a predefined template.

Strengths:

- Keeps runtime deterministic.
- Encourages reusable, tested flow templates.
- Makes lane behavior easier to reason about.

Weaknesses:

- Requires a useful template library.
- Less flexible for unusual projects.

### Option B: Solution Design Generates Custom Flow Graphs

Agents create a bespoke graph for each WBS.

Strengths:

- Very flexible.
- Could tailor orchestration to complex project shape.

Weaknesses:

- Higher validation burden.
- Greater risk of invalid or unsafe flows.
- Harder to compare runs.

### Option C: Template Plus Patch

WBS binds to templates but can request constrained variations.

Strengths:

- Middle path between reuse and flexibility.
- Good future direction once templates mature.

Weaknesses:

- Needs a patch model and compatibility checks.

### Working Recommendation

Use registered Flow Templates first. Revisit constrained template patching after the local engine can validate and execute templates reliably.

## Design Axis 5: Build And Eval Relationship

### Option A: Eval After Build Only

Eval begins once Build is complete.

Strengths:

- Simple lifecycle.
- Eval can inspect actual outputs before planning.

Weaknesses:

- Eval strategy may overfit to implementation.
- Weaker adversarial independence.

### Option B: Eval Checkpoints During Build

Eval critiques intermediate artifacts at defined gates.

Strengths:

- Faster feedback.
- Useful for large efforts.

Weaknesses:

- More synchronization complexity.
- Can slow Build and create coupling.

### Option C: Eval Strategy At Start, Execution At End

Eval generates its test/review strategy from the Demand Package, architecture, and WBS before Build output exists. It executes after Build artifacts are frozen.

Strengths:

- Strong adversarial posture.
- Clear separation from Build.
- Keeps orchestration simpler than checkpoint evals.

Weaknesses:

- Eval may miss implementation-specific risks.
- May need a later supplement for generated diff or code review checks.

### Working Recommendation

Use strategy-at-start and execution-at-end as the default. Later, allow optional checkpoint evals for `recursive` lane work.

## Design Axis 6: Verdict Semantics

### Option A: Binary Pass/Fail

Strengths:

- Simple.
- Easy to automate.

Weaknesses:

- Cannot represent missing evidence or blocked checks.
- Encourages false certainty.

### Option B: Multi-State Verdict

Use `pass`, `fail`, `inconclusive`, and `blocked`.

Strengths:

- Better represents agent and tooling uncertainty.
- Lets the system distinguish failure from lack of evidence.

Weaknesses:

- Requires policy for how overall verdicts aggregate.

### Working Recommendation

Use multi-state verdicts with evidence requirements tied back to acceptance criteria.

## Design Axis 7: Lane Selection

### Option A: Deterministic Rules

Select lanes from labels, size thresholds, or explicit package metadata.

Strengths:

- Predictable.
- Easy to test.

Weaknesses:

- Brittle for nuanced complexity.

### Option B: Agent Assessment

An agent reads the Demand Package and selects a lane with rationale.

Strengths:

- Handles nuance and ambiguity.
- Can explain the complexity signals.

Weaknesses:

- Less deterministic.
- Needs guardrails and reviewability.

### Option C: Hybrid

Use deterministic guardrails plus agent rationale.

Strengths:

- Better balance of repeatability and judgment.
- Can flag when agent selection violates policy.

Weaknesses:

- More machinery than either approach alone.

### Working Recommendation

Use a hybrid selector: deterministic thresholds and allowed lanes, with an agent producing the rationale and final recommendation inside that boundary.

## Design Axis 8: Agent Harness Strategy

### Option A: Build On Existing Agent Framework

Examples include graph-based or role-based agent frameworks.

Strengths:

- Faster initial experimentation.
- Existing abstractions for agents, tools, state, and memory.

Weaknesses:

- DFT may inherit mismatched assumptions.

- Frameworks may center agent autonomy more than deterministic orchestration.

### Option B: Custom Core With Adapters

Build DFT's orchestration core directly, then integrate agent frameworks or CLI tools behind runner adapters.

Strengths:

- Core matches DFT's artifact and flow model.
- Keeps provider and framework lock-in low.

Weaknesses:

- More engineering work.
- Need to design durable execution, logging, and retries carefully.

### Option C: Workflow Engine As Core

Use a durable workflow engine and implement DFT artifacts and runners on top.

Strengths:

- Strong retries, state, observability, and scheduling.

Weaknesses:

- May be heavy for a local-first toolkit.
- Can impose operational complexity too early.

### Working Recommendation

Keep this open until a short architecture spike compares a custom core, a graph-agent framework, and a workflow-engine substrate against DFT's requirements.

## Design Axis 9: Runtime Stack

Runtime should be chosen after the engine responsibilities are agreed.

### Option A: Python

Strengths:

- Strong YAML, schema, CLI, process, and AI ecosystem.
- Fast iteration for local tooling.

Weaknesses:

- Packaging and dependency isolation need care.
- Concurrency model may constrain some execution designs.

### Option B: TypeScript

Strengths:

- Strong fit for VS Code, Node CLIs, and web UI work.

- Good type ergonomics for artifact models.

Weaknesses:

- Process orchestration and Python-heavy AI tooling may require more bridges.

### Option C: Go Or Rust

Strengths:

- Good for a small static binary, strong concurrency, and robust CLIs.

Weaknesses:

- Slower iteration for agent harness experimentation.

- More work for schema-heavy product exploration.

### Working Recommendation

Do not decide yet. Choose runtime after deciding whether DFT is primarily an artifact compiler, a local execution harness, a VS Code-integrated tool, or a durable workflow runtime.

## Design Axis 10: Storage And State

### Option A: Filesystem Run Directories

Strengths:

- Easy to inspect, diff, archive, and debug.

- Good fit for local-first development.

Weaknesses:

- Querying run history can be clumsy.

- Concurrency and locking need care.

### Option B: SQLite Store

Strengths:

- Better run queries, indexing, and transactional updates.

- Still local-first.

Weaknesses:

- Artifacts still need file or blob handling.

- Less transparent than plain files.

### Option C: External Service Store

Strengths:

- Better for distributed execution and teams.

Weaknesses:

- Too much operational weight for the first local engine.

### Working Recommendation

Start conceptually with filesystem artifacts plus an append-only event log. Decide later whether the event log is plain JSONL, SQLite-backed, or both.

## Design Axis 11: Model Configuration

### Option A: Global Model Only

Strengths:

- Simple.

Weaknesses:

- Cannot tune models by task type.

### Option B: Hierarchical Overrides

Allow model defaults and overrides at global, run, lane, flow, and step levels.

Strengths:

- Matches varied needs of drafting, architecture, coding, and eval.

- Keeps orchestration configurable.

Weaknesses:

- Requires clear precedence rules.

### Working Recommendation

Use hierarchical model resolution with explicit precedence and run logs that capture the resolved model for each agent step.

## Design Axis 12: Chat-Session Execution

The prompt asks whether flows should also run entirely inside a chat session.

### Option A: Chat As Primary Runner

Strengths:

- Very easy to start.

- Preserves human steering.

Weaknesses:

- Hard to automate, replay, and validate.

### Option B: Chat As A Front Door Only

Chat helps author artifacts, then the engine runs outside the chat.

Strengths:

- Clean separation of interactive discovery and execution.

Weaknesses:

- Requires users to shift modes.

### Option C: Chat As An Adapter

The same flow concepts can be executed in a chat session for exploratory runs, but production runs use the engine.

Strengths:

- Flexible without making chat the core runtime.

Weaknesses:

- Requires adapter semantics and limitations.

### Working Recommendation

Treat chat as a front door first and a possible adapter later. The core run model should not depend on chat transcript state.

## Design Questions To Resolve Before Code

1. Is DFT primarily artifact-first, harness-first, or a balanced hybrid?
2. Should the first engine be a custom core, an existing graph framework, or a workflow engine substrate?
3. What is the minimum useful local run: Demand Package to WBS, WBS to Build, or WBS to Eval verdict?
4. Should storage be plain filesystem, SQLite, or filesystem plus event log?
5. How much should the initial flow model support: pure DAG only, or DAG plus bounded decision nodes?
6. What exact products or repositories are meant by OpenClaw and Hermes, and which design elements should be evaluated from them?
7. Should the first user interface be CLI, VS Code commands, chat prompt files, or just documented artifacts?

## Design Record Template

Each settled choice should be captured as a short design record:

```markdown
# DR-NNN: Title

## Status

Proposed | Accepted | Superseded

## Context

What pressure, constraint, or user need creates this choice?

## Options

What alternatives were considered?

## Decision

What are we choosing now?

## Consequences

What gets easier, harder, deferred, or constrained?
```