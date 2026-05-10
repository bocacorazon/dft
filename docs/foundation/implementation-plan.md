# DFT Implementation Plan

## Purpose

This plan begins after the design discussion, not before it. It is intentionally organized around decision gates and reversible implementation slices so the project does not prematurely commit to a runtime stack, framework, or storage substrate.

The plan assumes the current working direction from [docs/foundation/design-discussion.md](design-discussion.md): deterministic orchestration, artifact-based phase boundaries, early adversarial eval planning, and local-first execution. Those are still design commitments, not language or framework commitments.

The product direction is an agency gradient: powerful agents for Intent, assisted agents for Solution Design, and fully automated Dark Factory execution after those artifacts are cemented.

## Decision Gates

### Gate 1: Product Center Of Gravity

Decision needed: artifact-first, harness-first, conversation-first, or explicit hybrid. The current product thesis is a hybrid with conversational/agentic Intent, artifact-mediated Solution Design, and automated Orchestration.

Why it matters:

- Determines whether schemas, runner internals, or chat ergonomics are built first.
- Shapes the first milestone and testing strategy.

Output:

- A design record that defines the product center.

### Gate 2: Flow Semantics

Decision needed: ordered list, explicit DAG, DAG plus decision nodes, or state machine.

Why it matters:

- Determines the DSL, validator, scheduler, and run report shape.

Output:

- A flow semantics design record.
- A revised version of [docs/flow-dsl.yaml](../flow-dsl.yaml) or a new flow template example.

### Gate 3: Engine Substrate

Decision needed: custom core, graph-agent framework, workflow engine, or hybrid adapter model.

Why it matters:

- Determines build effort, extensibility, and dependency risk.

Output:

- A short spike comparing at least three candidate substrates against DFT requirements.

### Gate 4: Runtime Stack

Decision needed: Python, TypeScript, Go/Rust, or another stack.

Why it matters:

- Determines package layout, CLI tooling, test framework, dependency story, and agent/tool integration approach.

Output:

- A runtime design record with explicit reasons and rejected options.

### Gate 5: State Store

Decision needed: filesystem, SQLite, filesystem plus event log, or other store.

Why it matters:

- Determines observability, replay, locking, and run history.

Output:

- A run artifact layout and event log contract.

## Milestone 0: Foundation Alignment

Goal: make the design space explicit and preserve choices as reviewable records.

Work items:

- Review [docs/foundation/design-discussion.md](design-discussion.md).
- Convert accepted working recommendations into design records.
- Mark unresolved axes as open questions.
- Update [docs/foundation/design.md](design.md) so it represents accepted architecture only, not tentative options.

Acceptance checks:

- The foundation docs clearly distinguish accepted decisions from open options.
- The implementation plan does not assume a runtime stack before Gate 4.
- Every major product requirement from [docs/dark-factory-toolkit.md](../dark-factory-toolkit.md) appears in the design discussion or plan.

## Milestone 1: Artifact Contracts

Goal: define the artifacts that connect Intent, Solution Design, and Orchestration.

Work items:

- Define or refine schemas for Demand Package, Architecture Blueprint, Eval Plan, WBS, Flow Template, Run Report, and Verdict.
- Create minimal examples for each artifact.
- Define traceability fields from acceptance criteria to WBS tasks, build outputs, eval checks, and verdict evidence.
- Decide whether schemas are authored as JSON Schema YAML, pure JSON Schema, typed code models generated from schema, or code-first types exported to schema.

Acceptance checks:

- Example artifacts validate against schemas.
- Acceptance criteria can be traced from Demand Package through WBS and Eval Plan to Verdict.
- Schema choices do not depend on the final runtime stack unless Gate 4 has been resolved.

## Milestone 2: Flow Model And Validation

Goal: turn the DSL sketch into a flow template contract.

Work items:

- Define stable step ids, dependencies, setup hooks, run actions, verification hooks, capture policy, retry policy, and model override fields.
- Specify valid runner types: `agent`, `tool`, `function`, and `workflow`.
- Define validation rules: unique ids, dependency references, acyclicity, unsupported runner types, invalid retry policy, missing required inputs, and invalid output bindings.
- Add examples for `streamlined`, `spec`, and `recursive` lanes.

Acceptance checks:

- Invalid flow examples produce precise validation errors.
- Valid flow examples can be topologically ordered.
- The flow model can represent the current [docs/flow-dsl.yaml](../flow-dsl.yaml) sketch without losing intent.

## Milestone 3: Engine Substrate Spike

Goal: choose whether to build a custom core or stand on a framework.

Candidates to compare:

- Custom lightweight core.
- Graph-oriented agent framework.
- General workflow engine.
- Existing CLI harness pattern inspired by prior work such as skrunner or spec-kit orchestration.

Evaluation criteria:

- Deterministic DAG execution.
- Artifact-first boundaries.
- Local-first ergonomics.
- Parallel Build/Eval lifecycle.
- Runner adapter flexibility.
- Model configuration and override support.
- Observability and replay.
- Dependency and lock-in risk.

Acceptance checks:

- A design record recommends one substrate approach.
- Rejected approaches include clear reasons.
- The chosen substrate can support Milestone 4 without reworking artifact contracts.

## Milestone 4: Runtime And Project Scaffold

Goal: create the implementation skeleton after the runtime stack is chosen.

Work items:

- Add project metadata and dependency management for the chosen stack.
- Add package/module layout.
- Add CLI or command entrypoint if selected as the first interface.
- Add test structure and fixture directories.
- Add schema loading and artifact validation helpers.

Acceptance checks:

- A developer can run the test suite.
- A developer can validate example artifacts from the command line or equivalent interface.
- No real LLM invocation is required for this milestone.

## Milestone 5: Local Run Store And Registry

Goal: make runs inspectable and repeatable.

Work items:

- Implement run directory or store layout.
- Store input artifacts, expanded flow templates, step outputs, logs, and reports.
- Implement an append-only event log.
- Implement registries for lanes, flow templates, runner adapters, tools, functions, workflows, and model policies.
- Implement model resolution precedence after the model configuration design is accepted.

Acceptance checks:

- A run has a stable id and inspectable artifacts.
- Event logs capture state transitions and runner decisions.
- Registry lookups fail with useful errors.

## Milestone 6: Local DAG Execution Without LLMs

Goal: prove deterministic orchestration before adding agent complexity.

Work items:

- Implement dependency scheduling.
- Support bounded local parallelism.
- Implement `function` runners with deterministic fixture functions.
- Implement safe `tool` runners for simple local commands if selected.
- Implement setup, verification, capture, retry, and failure policy semantics.

Acceptance checks:

- A fixture WBS maps to a fixture Flow Template and executes successfully.
- Cycle detection and failed dependency behavior are tested.
- A final run report links outputs back to WBS tasks.

## Milestone 7: Build And Eval Lifecycle

Goal: prove the parallel Build/Eval model.

Work items:

- Start Eval planning at orchestration start.
- Freeze Build artifacts after Build completion.
- Execute eval suites after artifacts are frozen.
- Emit pass, fail, inconclusive, or blocked verdicts with evidence.
- Map eval evidence to acceptance criteria.

Acceptance checks:

- Eval strategy is created before Build results exist.
- Eval execution cannot mutate Build artifacts.
- Final verdict traces to acceptance criteria.

## Milestone 8: Agent Adapter Boundary

Goal: add agents without making the core depend on one provider or chat session.

Work items:

- Define the `agent` runner adapter interface.
- Add dry-run, echo, or fixture agent adapter.
- Add transcript capture and resolved model logging.
- Later, add real CLI-based agent adapters.

Acceptance checks:

- The same flow can run with a fixture agent and later a real adapter.
- Agent outputs are captured as artifacts.
- The core can validate and report failures without provider-specific logic.

## Milestone 9: Solution Design Pipeline

Goal: implement the agents that produce orchestration inputs.

Work items:

- Add Architecture Design Agent prompt/interface.
- Add Eval Criteria Conversion Agent prompt/interface.
- Add Lane Selector Agent prompt/interface.
- Add WBS Generator prompt/interface.
- Run them first with fixture adapters, then real agent adapters.

Acceptance checks:

- A Demand Package can produce an Architecture Blueprint, Eval Plan, Lane selection, and WBS.
- The WBS only binds to registered Flow Templates.
- Lane selection includes complexity rationale and evidence.

## Milestone 10: Complete Example

Goal: demonstrate one end-to-end path.

Work items:

- Add a minimal Demand Package example.
- Generate or provide Solution Design artifacts.
- Execute a local Build flow with fixture runners.
- Execute Eval and produce a verdict.
- Document the full run.

Acceptance checks:

- One command or documented sequence runs the example.
- The final report contains artifacts, logs, step outcomes, eval evidence, and verdict.

## Deferred Work

- Docker execution backend.
- Kubernetes execution backend.
- GitHub Actions backend.
- Full chat-session execution mode.
- Dynamic agent-generated flow graphs.
- Rich UI.
- Team/server persistence.
- Real coding-agent safety and workspace isolation beyond local prototype needs.