# DFT Product Design

## Purpose

This document turns the high-level Dark Factory Toolkit vision into product design options. It focuses on where to use existing agent harnesses, where DFT should own its own control plane, and what best practices should shape each phase.

The key product thesis is an agency gradient:

- Intent is agent-rich and conversational.
- Solution Design is agent-assisted and reviewable.
- Orchestration is automated Dark Factory execution.

DFT should not maximize autonomy everywhere. It should use autonomy where ambiguity is high, then progressively convert that autonomy into typed artifacts, deterministic execution, evidence, and verdicts.

## State Of The Art Summary

Current agent and workflow products tend to separate control flow from agent work.

- CrewAI separates Flows from Crews: Flows manage state and control, while Crews perform collaborative agent work.
- AutoGen separates lower-level Core concepts from AgentChat and Studio: useful for teams, termination conditions, handoffs, and human feedback.
- OpenHands separates SDK, CLI, local GUI, headless mode, GitHub Action, sandboxing, and cloud/enterprise surfaces.
- Workflow engines such as Temporal, Prefect, and Dagster emphasize durable execution, retries, state recovery, lineage, observability, and operational confidence.
- LLM application frameworks such as LlamaIndex and Haystack emphasize typed state, tools, pipelines/workflows, MCP/tool integration, observability, and evaluation hooks.
- LLM product best practice is eval-first development: define success criteria and empirical tests before prompt tuning.

The pattern for DFT is clear: DFT should own product state, artifacts, policies, and phase transitions; harnesses should be replaceable workers behind adapters.

## Product Strategy Options

### Option 1: Use OpenClaw Or Similar As The Whole Process Harness

DFT would primarily be conventions, prompts, and skills running inside OpenClaw or a comparable personal AI assistant ecosystem.

Strengths:

- Fastest path to using powerful existing agents.
- May inherit model abstraction, skills, MCP-style tooling, and assistant surfaces.
- Useful for Intent exploration and day-to-day interactive work.

Weaknesses:

- DFT's phase boundaries may become conventions rather than enforceable product contracts.
- Harder to guarantee Demand Package, WBS, eval plan, run report, and verdict traceability.
- Dark Factory execution may inherit assistant-session behavior instead of becoming a deterministic run model.
- OpenClaw should be pinned to exact repositories and docs before becoming a core dependency.

Best fit:

- Intent Studio front door.
- Skill ecosystem inspiration.
- Optional adapter for interactive agent sessions.

### Option 2: Use A Coding Agent Harness As The Build Engine

DFT would use a software-agent harness such as OpenHands for code-writing execution while retaining separate Intent and Solution Design artifacts.

Strengths:

- Strong fit for actual software changes.
- Existing sandbox, CLI, headless automation, JSONL event streams, GitHub Action integration, model configuration, and coding-agent ergonomics.
- Reduces the need to build a coding agent from scratch.

Weaknesses:

- The harness may expect one task prompt rather than a typed DFT task package.
- Build/Eval separation must be imposed externally.
- DFT still needs its own artifact ledger, lane model, WBS binding, eval planning, and verdict semantics.

Best fit:

- Build runner adapter for Dark Factory steps.
- Headless execution worker.
- Sandbox and event-stream reference design.

### Option 3: Use A Multi-Agent Framework As The Product Core

DFT would be built mostly on a framework such as CrewAI, AutoGen, LangGraph, or a similar agent workflow library.

Strengths:

- Excellent fit for Intent and Solution Design agent teams.
- Existing role/task/team patterns, human feedback, state, termination conditions, and structured output support.
- Faster than building all collaboration mechanics directly.

Weaknesses:

- Frameworks may center agent autonomy more than DFT wants in orchestration.
- DFT could inherit framework-specific state, persistence, and execution semantics.
- Later Docker/Kubernetes/GitHub Actions backends may still require a separate control plane.

Best fit:

- Intent and Solution Design agent groups.
- Bounded agent nodes inside a DFT-controlled workflow.
- Prototyping prompts, agent roles, and validation rubrics.

### Option 4: Use A Workflow Engine As The Product Core

DFT would build on a durable workflow engine and treat agents/tools as activities.

Strengths:

- Best-in-class execution durability, retries, recovery, state, and observability.
- Strong fit for unattended Dark Factory runs.
- Clear operational model for long-running processes.

Weaknesses:

- Heavy before DFT's artifact contracts are stable.
- Not agent-native for Intent discovery.
- Could make the product feel like infrastructure before the product shape is settled.

Best fit:

- Later durable execution substrate.
- Inspiration for retries, event logs, state recovery, and observability.
- Possible backend once local execution requirements are proven.

### Option 5: Build DFT As A Control Plane With Harness Adapters

DFT owns artifacts, phase transitions, policy, run state, eval contracts, and orchestration semantics. Existing harnesses run behind adapters where useful.

Strengths:

- Preserves the DFT product vision.
- Lets each phase use the right worker technology.
- Keeps OpenClaw, OpenHands, CrewAI, AutoGen, and future tools replaceable.
- Makes artifacts and verdicts first-class instead of hidden transcript conventions.
- Supports future execution backends without rewriting Intent and Solution Design.

Weaknesses:

- More product design and adapter work.
- Requires careful contracts for artifacts, events, runners, and evidence.
- Slower than adopting one harness wholesale.

Best fit:

- Recommended product architecture.

## Recommended Product Architecture

DFT should be a control plane with five layers.

### 1. Intent Studio

Intent Studio is the agent-rich environment where a human develops intention. It may use chat, documents, structured interviews, and critique loops.

Primary output:

- Demand Package.

Responsibilities:

- Conduct Socratic discovery.
- Challenge goals and assumptions.
- Extract constraints, non-goals, risks, stakeholders, and success criteria.
- Generate strong acceptance criteria.
- Validate completeness and ambiguity.
- Compile strict structured output.

### 2. Solution Design Studio

Solution Design Studio translates the Demand Package into architecture and execution-planning artifacts.

Primary outputs:

- Architecture Blueprint.
- Eval Plan.
- Lane Selection.
- WBS.

Responsibilities:

- Design architecture boundaries and interfaces.
- Convert acceptance criteria into eval strategies and evidence requirements.
- Select lane by complexity.
- Decompose work into WBS tasks.
- Bind WBS tasks to registered Flow Templates.

### 3. Artifact Ledger

The Artifact Ledger is the product memory. It stores phase outputs, design records, run inputs, run outputs, evidence, and verdicts.

Responsibilities:

- Version artifacts.
- Preserve traceability.
- Support diffing and review.
- Store run reports and event logs.
- Keep the system reproducible even if agent conversations are discarded.

### 4. Orchestration Control Plane

The control plane executes the selected lane and flow templates. It should be deterministic and policy-driven.

Responsibilities:

- Expand WBS tasks into Flow Template runs.
- Resolve model and runner policy.
- Schedule steps.
- Start Build and Eval lifecycles.
- Freeze artifacts before evaluation.
- Aggregate evidence into a verdict.

### 5. Runner Adapter Layer

Runner adapters invoke external systems without letting them own DFT's product semantics.

Candidate adapters:

- `agent-team`: CrewAI, AutoGen, LangGraph, or similar.
- `coding-agent`: OpenHands, OpenClaw, Codex, Claude Code, or similar.
- `tool`: shell commands, MCP servers, repository tools, issue trackers.
- `function`: deterministic local functions.
- `workflow`: Temporal, Prefect, Dagster, GitHub Actions, or nested DFT flow.

## Harness Evaluation Matrix

| Candidate | Strongest Fit | Weak Fit | DFT Use |
| --- | --- | --- | --- |
| OpenClaw | Personal assistant, skills, model/tool surface | Unpinned as orchestration substrate | Intent surface or skill adapter after a repo/docs spike |
| OpenHands | Coding-agent execution, sandboxing, headless mode, JSONL events | Product-wide artifact governance | Build runner adapter |
| CrewAI | Agent teams plus controlled flows, structured outputs, state | Deterministic DFT runtime if used wholesale | Intent/Solution Design crews or bounded flow nodes |
| AutoGen | Multi-agent conversation, termination, handoffs, human feedback | Artifact-led orchestration without extra layer | Intent critique loops and design review teams |
| LangGraph | Stateful graph execution and agent workflows | May still require DFT artifact/control layer | Candidate graph substrate spike |
| LlamaIndex/Haystack | Tooling, typed state, RAG, observability, MCP-style integration | Full software build orchestration | Knowledge/research/tool adapters |
| Temporal | Durable execution, recovery, long-running workflows | Heavy for early local-first exploration | Future durable orchestration backend |
| Prefect | Pythonic local-to-cloud orchestration, retries, observability | Dynamic Python flow may compete with DFT DSL | Possible local workflow backend spike |
| Dagster | Asset lineage, observability, testability | Data/asset orientation may not match all DFT work | Inspiration for lineage and artifact thinking |

## Phase Best Practices

### Intent Phase

Recommended agent set:

- Intent Interviewer: conducts Socratic discovery.
- Product Critic: identifies weak goals, ambiguous language, and hidden assumptions.
- Domain/User Advocate: tests whether the intent maps to real user value.
- Constraint Miner: extracts technical, operational, security, compliance, cost, and timeline constraints.
- Acceptance Criteria Engineer: turns goals into verifiable conditions.
- Completeness Validator: checks the draft against a rubric.
- Demand Package Compiler: emits strict structured output.

Best practices:

- Keep transcript, draft document, validation findings, and final Demand Package separate.
- Require non-goals and out-of-scope items.
- Require every acceptance criterion to include an evidence expectation.
- Preserve unresolved questions instead of hiding them.
- Use adversarial validators against the drafting agent.
- Treat Demand Package acceptance as a formal gate.

Harness options:

- Use CrewAI or AutoGen for multi-agent interviews and critique loops.
- Use OpenClaw-like assistant surfaces for interactive UX and skills.
- Keep the final Demand Package compiler under DFT contract validation.

### Solution Design Phase

Recommended agent set:

- Architecture Design Agent: produces architecture blueprints and technical decisions.
- Technical Critic: reviews risks, complexity, coupling, security, and maintainability.
- Eval Criteria Converter: maps acceptance criteria to eval suites and evidence rules.
- Lane Selector: selects `streamlined`, `spec`, or `recursive` using complexity signals.
- WBS Generator: decomposes work into conceptual tasks.
- Flow Binder: maps WBS tasks to registered Flow Templates.

Best practices:

- Keep architecture decisions as reviewable design records.
- Separate architecture blueprint from execution plan.
- Ensure every WBS task traces to one or more acceptance criteria.
- Generate eval plans before Build output exists.
- Use hybrid lane selection: deterministic rubric plus agent rationale.
- Require human review before entering Dark Factory execution unless a lane explicitly permits auto-approval.

Harness options:

- Use CrewAI/AutoGen teams for architect/critic/evaluator collaboration.
- Use LangGraph or similar graph tools if design workflows need explicit state and checkpoints.
- Keep WBS-to-template binding inside DFT so agents cannot invent runtime control flow without validation.

### Orchestration Phase

Recommended components:

- WBS Expander: resolves WBS tasks to Flow Templates.
- Scheduler: executes deterministic step dependencies.
- Build Thread: runs implementation steps.
- Eval Thread: creates an adversarial eval plan at start, then evaluates frozen Build artifacts.
- Artifact Freezer: snapshots outputs before evaluation.
- Verdict Aggregator: maps evidence to acceptance criteria and final status.

Best practices:

- No manager agent should decide runtime routing in the default path.
- Use sandboxing for any code-writing or shell-running worker.
- Capture structured events for every runner action.
- Preserve model choice, inputs, outputs, retries, costs, and failures.
- Make every step resumable or explicitly non-resumable.
- Distinguish `pass`, `fail`, `inconclusive`, and `blocked`.
- Treat logs and evidence as product artifacts.

Harness options:

- Use OpenHands or similar for coding-agent Build steps.
- Use deterministic tool/function runners for simple steps.
- Use workflow engines later for durable execution.
- Use DFT's control plane for lifecycle, policies, and verdicts.

## Overall Process Options

### Option A: Monolithic Agent Session

One agent or assistant takes the project from intent to code.

Use when:

- Work is tiny or exploratory.

Avoid when:

- You need reproducibility, evidence, or unattended execution.

### Option B: Phase-Gated Agent Teams

Each phase uses specialized agents and emits typed artifacts.

Use when:

- The goal is spec-driven development with reviewable transitions.

Avoid when:

- The overhead is too high for a tiny fix.

### Option C: DFT Control Plane With Pluggable Workers

DFT owns phase gates and artifacts; workers perform bounded tasks.

Use when:

- You want a long-lived product, not a prompt convention.

Avoid when:

- You only need a short-lived experiment.

## Recommended Next Product Decisions

1. Define the first-class product object: Demand Package, Workspace, Run, or Increment.
2. Choose the first user experience for Intent Studio: chat-first, document-first, or hybrid.
3. Decide whether Solution Design always requires human approval before Dark Factory execution.
4. Define the minimum Dark Factory run: one WBS task, one feature, or one full increment.
5. Pin the exact OpenClaw and Hermes repositories/products to evaluate.
6. Choose the first harness spike: OpenHands as Build adapter, CrewAI/AutoGen as Intent/Solution agent teams, or workflow engine substrate.

## Working Recommendation

Build DFT as a product control plane with harness adapters.

Do not make any single agent harness the whole process yet. Use agent-team frameworks where collaboration is valuable, coding-agent harnesses where code execution is valuable, and workflow-engine ideas where durability is valuable. DFT should own the contracts that matter most: artifacts, gates, lanes, policies, evidence, and verdicts.