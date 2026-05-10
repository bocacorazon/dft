# Dark Factory Toolkit Foundation Architecture

## Purpose

Dark Factory Toolkit (DFT) is a CLI-first system for running a spec-driven development process from intent through design, orchestration, build, evaluation, and final delivery.

DFT should not begin as a general-purpose agent runtime. Its first architectural responsibility is to own the development workflow, durable artifacts, validation gates, and execution records while invoking external agents, tools, and runtimes through adapters.

## Design principles

1. **Workflow over agent runtime**: DFT owns the spec-driven lifecycle; agent CLIs and automation tools remain replaceable executors.
2. **Repo-local artifacts**: Human-readable project artifacts live in the repository so they can be reviewed, versioned, and evolved with the code.
3. **Durable run state**: Execution state, logs, retries, step status, eval results, and verdicts live in a local run database.
4. **Deterministic orchestration**: Approved WBS artifacts compile into explicit DAGs. Agents may propose changes, but DFT should not allow silent graph mutation.
5. **Adversarial evaluation**: Eval plans are generated from the accepted demand package, not from the build plan, so they remain anchored in original intent and acceptance criteria.
6. **Human gates before autonomy**: Demand acceptance and WBS acceptance are hard human approval gates. After WBS approval, the compiled flow should run automatically unless it hits an exception, policy gate, or proposed graph change.
7. **Implementation-neutral foundation**: The architecture should define stable contracts before committing to a language stack. Go, Python, or a hybrid approach can be evaluated later against these contracts.

## Inspiration from existing systems

DFT should borrow selectively from existing agent orchestration systems without becoming a clone of any one of them.

| System family | Useful ideas for DFT | What DFT should avoid |
| --- | --- | --- |
| LangGraph-style runtimes | Durable execution, stateful graphs, resumability, human interrupts, persistent state | Tying DFT's product model to one framework's graph abstraction |
| AutoGen-style multi-agent frameworks | Event-driven agents, tool extensions, Docker-backed code execution, multi-agent experiments | Making conversation between agents the primary artifact |
| OpenClaw-style mission control tools | Boards, approvals, audit history, gateway-aware execution, operations visibility | Starting with a heavy web control plane before the CLI workflow is proven |
| Hermes-style agent platforms | Single-command UX, model/provider configuration, tool adapters, memory, terminal backends, telemetry | Building a full standalone agent platform before DFT's spec workflow is validated |
| Spec-driven development tools | Demand/spec artifacts, acceptance criteria, implementation plans, validation loops | Treating generated specs as one-off documents instead of durable workflow inputs |

## Product model

The top-level unit of work is an **Increment**. An Increment is a bounded software change that moves through the full DFT lifecycle.

An Increment contains:

- a demand package
- solution design artifacts
- lane selection
- WBS artifacts
- compiled flow definitions
- build run records
- eval plans and eval run records
- final verdict and delivery notes

## Lifecycle

```text
Intent session
  -> demand package
  -> human acceptance gate
  -> solution design
  -> lane selection
  -> WBS
  -> human acceptance gate
  -> flow compilation
  -> build execution
  -> eval execution
  -> verdict
  -> delivery package
```

### Intent

The intent phase helps the user describe the desired software change. Its output is an accepted demand package containing:

- problem statement
- goals and non-goals
- target users or consumers
- constraints
- assumptions
- acceptance criteria
- evaluation guidance

The demand package is the root artifact for downstream design and eval.

### Solution design

The solution design phase turns the accepted demand package into architecture and implementation strategy. It should produce:

- architecture blueprint
- major design decisions
- impacted components
- risk notes
- validation strategy
- initial lane recommendation

### Lane selection

DFT should support three first-class lanes at the foundation level:

| Lane | Use when | Expected structure |
| --- | --- | --- |
| `streamlined` | Small, well-understood changes | Lightweight design, compact WBS, short flow |
| `spec` | Normal feature or product increments | Full demand, design, WBS, build, and eval flow |
| `recursive` | Large or ambiguous work requiring decomposition | Parent increment that creates child increments/specs |

The lane selector is a policy component. It should recommend a lane using demand size, ambiguity, risk, affected surfaces, and acceptance criteria complexity. The user can override the lane before WBS acceptance.

### WBS

The WBS phase produces two related artifacts:

1. A human-readable work breakdown describing specs, dependencies, risk, and sequencing.
2. A machine-readable WBS document used to compile orchestration flows.

The WBS is a hard approval gate. Once accepted, DFT compiles it into deterministic build and eval flows.

### Orchestration

The orchestration phase executes a compiled flow DAG. In the first architecture, DFT should focus on the local execution engine.

The local engine should support:

- DAG execution
- step dependencies
- parallel execution where dependencies allow it
- retries according to policy
- resumability
- cancellation
- structured logs
- artifact capture
- model and adapter configuration
- explicit failure states

Agents and tools are invoked through adapters. An adapter can wrap an agent CLI, local command, script, function, nested workflow, or future remote executor.

### Build and eval relationship

When WBS execution begins, DFT should start from two plans:

- **Build plan**: compiled from the accepted WBS.
- **Eval plan**: generated from the accepted demand package and acceptance criteria.

The eval plan should not be derived from the build plan. This preserves an adversarial relationship: Build optimizes for implementation, Eval checks whether the original demand was actually satisfied.

The eval thread can run checks as build outputs become available, but its source of truth remains the demand package.

## High-level architecture

```text
CLI
  |
  v
Session controller
  |
  +--> Artifact store
  |      demand / design / WBS / flows / eval plans / verdicts
  |
  +--> Policy services
  |      lane selector / approval gates / model selection / risk rules
  |
  +--> Flow compiler
  |      WBS -> executable DAGs
  |
  +--> Local orchestrator
  |      scheduler / process runner / retry manager / state machine
  |
  +--> Adapter layer
  |      agents / tools / functions / workflows / future executors
  |
  +--> Eval engine
  |      eval plan generation / eval execution / verdict aggregation
  |
  +--> Run database
         runs / steps / logs / artifacts / metrics / verdicts
```

## Core components

### CLI

The CLI is the primary first-version user interface. It should optimize for guided interactive sessions that create and update artifacts.

Potential command shape:

```text
dft init
dft intent start
dft intent accept
dft design start
dft wbs create
dft wbs accept
dft flow compile
dft run
dft eval
dft status
dft inspect
```

The exact command names can evolve. The stable concept is that interactive commands guide the user through artifact creation and approval, while non-interactive flags can be added later for automation.

### Artifact store

The artifact store is repo-local and file-based. It should contain durable, human-readable files for each Increment.

Candidate layout:

```text
.dft/
  increments/
    <increment-slug>/
      demand.yaml
      demand.md
      design.md
      lane.yaml
      wbs.md
      wbs.yaml
      flows/
        build.yaml
        eval.yaml
      eval/
        plan.yaml
      verdict.md
```

The exact location can be revisited. If project-facing artifacts should be more visible, selected files can live under `docs/` while `.dft/` stores operational metadata.

### Run database

The run database stores operational state that should not be hand-edited.

It should track:

- increments
- runs
- flow versions
- step status
- step attempts
- process metadata
- captured outputs
- logs
- artifacts
- eval results
- final verdicts

SQLite is a natural first candidate, but the design should only require an embedded local database with transactional updates.

### Flow compiler

The flow compiler turns accepted WBS artifacts into executable DAGs.

Responsibilities:

- validate WBS schema
- resolve dependencies
- select lane templates
- expand reusable workflow steps
- bind adapters
- apply model/provider configuration
- emit executable flow definitions
- detect cycles and missing inputs

### Local orchestrator

The first execution engine should be local.

Responsibilities:

- execute DAG steps
- coordinate parallel work
- invoke adapters
- capture stdout, stderr, structured output, and produced files
- persist state transitions
- resume interrupted runs
- enforce retry and failure policies
- surface exceptions and graph-change proposals

Future executors, such as Docker, Kubernetes, or GitHub Actions, should be modeled as alternate backends behind the same orchestration contract.

### Adapter layer

Adapters isolate DFT from specific agent or tool implementations.

Initial adapter types:

- `agent`: invokes an LLM agent CLI or API
- `tool`: invokes an external command
- `function`: invokes a local built-in capability
- `workflow`: invokes another DFT flow

Adapters should declare:

- inputs
- outputs
- environment requirements
- model/provider preferences
- permissions
- timeout and retry policy
- output capture contract

### Eval engine

The eval engine owns adversarial validation.

Responsibilities:

- generate eval plan from demand package and acceptance criteria
- create machine-checkable assertions where possible
- create review prompts where judgment is required
- run tests, linters, static checks, or custom commands
- invoke evaluator agents where appropriate
- aggregate results into a verdict
- map failed evals back to acceptance criteria

## Flow DSL direction

The current draft in `docs/flow-dsl.yaml` captures the right basic idea: a flow with steps, setup, execution, capture, verification, and error behavior.

The foundation design should evolve the DSL toward:

- explicit step IDs
- explicit dependencies
- typed inputs and outputs
- adapter binding
- retry and timeout policy
- permissions
- artifact capture rules
- model/provider overrides
- conditionals only where necessary
- machine-readable status and result schemas

Example direction:

```yaml
flow:
  id: build.increment-slug
  lane: spec
  steps:
    - id: implement.api
      type: agent
      adapter: copilot-cli
      depends_on: []
      inputs:
        - wbs.yaml
        - design.md
      outputs:
        - src/**
        - tests/**
      model:
        preference: coding-default
      retry:
        max_attempts: 1
      capture:
        stdout: true
        artifacts: true
```

## Open decisions

- Implementation stack: Go, Python, TypeScript, Rust, or hybrid.
- Exact repo-local artifact layout.
- Whether `.dft/` or `docs/` is the primary user-facing artifact location.
- Initial schema language for demand, WBS, flow, and eval documents.
- How much of spec-kit should be wrapped directly versus treated as an external adapter.
- How model/provider configuration should be represented.
- Whether the first eval engine should be command-based, agent-based, or mixed.

## Recommended first development slice

The first useful vertical slice should prove the product architecture without building a full platform:

1. Create an Increment.
2. Guide a user through a demand package.
3. Accept the demand package.
4. Select a lane.
5. Generate a WBS.
6. Accept the WBS.
7. Compile a simple local flow DAG.
8. Execute local command or agent-adapter steps.
9. Generate an eval plan from the demand package.
10. Run eval steps.
11. Store run state and produce a verdict.

This slice validates DFT's core claim: a CLI-first, artifact-centered system can coordinate spec-driven development from intent to evaluated output while external agents remain replaceable.
