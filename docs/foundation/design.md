# Dark Factory Toolkit (DFT) - Foundation Design

## 1. Executive Summary & Vision
The Dark Factory Toolkit (DFT) is an orchestration engine designed to fully embrace spec-driven development. It provides a robust, multi-agent toolchain that transitions a project from an initial human "Intent" down to a validated software "Artifact" via structured, automated pipelines. By enforcing strict schemas and parallelizing the build and evaluation processes, DFT ensures high-quality output aligned precisely with the original requirements.

## 2. High-Level Architecture
The system operates across three primary phases:
1. **Intent Phase**: Converts human requirements into a strict "Demand Package."
2. **Solution Design Phase**: Converts the Demand Package into architecture blueprints, evaluation criteria, and a concrete Work Breakdown Structure (WBS).
3. **Orchestration Phase**: Executes the WBS using a rigid Directed Acyclic Graph (DAG) parser, spawning parallel threads for building and evaluating the software.

## 3. Phase 1: Intent (The Demand Package)
The intent phase is responsible for capturing the user's requirements and translating them into a highly structured, machine-actionable format known as the **Demand Package**.

- **Drafting Workflow**: A conversational agent interacts with the user to draft an initial document detailing the software artifact, features, and constraints.
- **Validation Loop**: Validator agents systematically review the drafted document for completeness and internal consistency.
- **Strict Output**: A generator agent compiles the validated draft into a strict YAML/JSON schema. The resulting Demand Package includes explicit inputs, constraints, and comprehensive acceptance criteria which serve as the definitive source of truth for the project.

## 4. Phase 2: Solution Design
In this phase, the Demand Package is dissected to produce the blueprints and operational plans necessary for construction.

- **Architecture Design Agent**: Analyzes the Demand Package to formulate system architecture blueprints and technical designs.
- **Eval Criteria Conversion Agent**: Translates the Demand Package's acceptance criteria into programmatic, machine-usable formats targeting the downstream adversarial Eval engine.
- **Lane Selector Agent**: Evaluates the package to determine project complexity. It routes the payload into an appropriate execution lane (e.g., 'spec', 'streamlined', 'recursive').
- **WBS Generator**: Translates the architecture, intent, and lane selection into a Work Breakdown Structure (WBS). The WBS consists of discrete conceptual steps mapped into predefined Flow Templates conforming to the `flow-dsl.yaml` schema.

## 5. Phase 3: Orchestration Engine
This phase conducts the actual construction. It takes the Flow Templates from the WBS and executes them.

- **Pluggable Execution Engine**: Built with a pluggable architecture. The initial implementation focuses on a **Local Engine** utilizing thread-based parallelism. Future backends may include Docker, Kubernetes, or GitHub Actions.
- **Execution Model**: Operates as a deterministic state engine. It parses and executes rigid DAGs defined in `flow-dsl.yaml`. LLMs are invoked strictly at the node level to execute specific prompts or verifications based on the YAML scaffolding, rather than dictating the flow itself.
- **Dual Threads (Build vs. Eval)**:
  - **Build Thread**: Executes the step-by-step artifact construction per the DAG.
  - **Eval Thread**: Operates *adversarially* and fully autonomously. It generates comprehensive evaluation suites at the very beginning of the orchestration phase. Once the Build thread concludes, the Eval thread executes its validation tests against the generated artifacts to produce a final verdict.
- **LLM Abstraction**: LLM integrations are fully configurable. Models can be dynamically assigned or overridden at the orchestrator or task level, avoiding lock-in and allowing step-specific model tuning.
