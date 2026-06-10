
Verification Layer — Landscape & Design Survey
Field	Value
Parent	Landscape Overview
Session	5 — Verification Layer Design
Status	Working Draft
Depends On	architecture.md §7, integration.md §8–9, ADR-004
1. The Core Problem
The Verification Engineer (VE) must independently verify implementations across programs with wildly varying interfaces: REST APIs, gRPC services, CLI tools, libraries (Python/Go/Java/JS), event-driven systems, web UIs, database schemas, infrastructure configurations, and more. No single testing tool or framework covers this diversity.

The Question
How can we create a generic verification layer that interacts with programs that have diverse interfaces, APIs, and SDKs — without requiring the VE to be rewritten for each target?

Constraints (from existing design)
Constraint	Source	Implication
Informational isolation	architecture.md §7	VE sees only committed artifacts + spec — never implementation reasoning
Spec-driven	integration.md §8	Tests derive from the spec, not the implementation
Language-agnostic	requirements.md VHIR-001	Cannot be tied to a single language ecosystem
Hidden eval support	integration.md §9	Must generate tests that agents cannot anticipate or game
Containerized execution	ADR-003	Tests run in isolated containers (Pool C, Kata-level)
Verdict standardization	integration.md §10	All results flow through a single Verdict Service with uniform schema
2. Interface Taxonomy
Programs that the VE must verify fall into distinct interface categories, each requiring different verification strategies:

Interface Type	Examples	Discovery Method	Primary Verification Pattern
HTTP/REST API	Microservices, web backends	OpenAPI/Swagger schema	Schema-driven fuzz + contract testing
GraphQL	API gateways, BFF layers	GraphQL introspection	Schema-driven query generation
gRPC	Internal services, ML pipelines	.proto files	Proto-driven contract + property testing
CLI Tool	Build tools, dev utilities	--help, man pages, spec	Invocation-based assertion testing
Library/SDK	Shared packages, utilities	Type signatures, docs, spec	Unit test generation + property testing
Web UI	Frontend apps, dashboards	DOM, accessibility tree	Browser automation (Playwright)
Event-Driven	Kafka consumers, queue handlers	Schema registry, AsyncAPI	Message contract testing
Database Schema	Migrations, stored procedures	DDL, migration files	Schema diff + data integrity tests
Infrastructure	Terraform, K8s manifests	HCL, YAML manifests	Policy-as-code (OPA, Conftest)
Composite	Full-stack features	Multiple of above	Multi-layer orchestrated verification
Key Insight
The "generic" part is NOT a single tool that handles all interfaces. It's an AI agent equipped with a toolkit of verification strategies that it composes based on the target program's characteristics. The agent reasons about WHAT to verify (from spec) and HOW to verify it (from interface analysis).

3. Landscape Survey — Verification Tools by Category
3.1 Schema-Driven API Testing
Tools that auto-generate tests from API schemas.

Schemathesis
Field	Value
Source	schemathesis/schemathesis
Language	Python (CLI + library)
Schema Support	OpenAPI 2.x/3.x, GraphQL
Approach	Property-based fuzzing from schema; generates thousands of test cases
Capabilities:

Finds 500 errors, schema violations, validation bypasses, stateful bugs
Stateful testing: sequences like create → get → update → delete
Hypothesis-powered (shrinking, reproducible counterexamples)
CI/CD integration (GitHub Action available)
Used by Spotify, WordPress, JetBrains, Red Hat
VE Relevance: HIGH — for any service with an OpenAPI or GraphQL schema, Schemathesis can be invoked programmatically by the VE agent to generate comprehensive fuzz tests. The VE doesn't need to write individual test cases — it feeds the schema and interprets the results.

Limitation: Requires a live running service and a valid schema. Does not help with libraries, CLIs, or UI testing.

Step CI
Field	Value
Source	stepci/stepci
Language	Node.js (CLI)
Protocol Support	REST, GraphQL, gRPC, tRPC, SOAP
Approach	YAML-defined multi-step API workflows with assertions
Capabilities:

Language-agnostic (YAML configuration)
Multi-protocol in a single workflow
Environment variable templating
Response validation with regex, JSON Schema, custom checks
VE Relevance: MEDIUM-HIGH — excellent for verifying API workflows that span multiple protocols. The VE agent can generate Step CI YAML programmatically from the spec, then execute it. The YAML format is LLM-friendly (easy to generate correctly).

Limitation: API-only. No library, CLI, or UI support.

Dredd (API Blueprint / OpenAPI)
Field	Value
Source	apiaryio/dredd
Status	Maintenance mode (not actively developed)
Approach	Validates live API against documentation
VE Relevance: LOW — superseded by Schemathesis for most use cases. Mentioned for completeness.

3.2 Contract Testing
Tools that verify interface compliance between services without end-to-end integration.

Pact
Field	Value
Source	pact-foundation (JS, JVM, .NET, Go, Python, Ruby, Rust, Swift)
Approach	Consumer-driven contract testing
Protocol Support	HTTP, messaging (Kafka, SNS, SQS, RabbitMQ)
Adoption	De-facto standard for contract testing
How It Works:

Consumer defines expected interactions (request → response pairs)
Pact framework generates a contract file (.pact.json)
Provider replays the contract against its actual implementation
Mismatches = breaking changes caught before deployment
Key Pattern — Ports & Adapters:
Pact encourages separating protocol-specific code (adapters) from domain logic (ports). The consumer test targets the port, not the adapter — making tests protocol-agnostic.

VE Relevance: HIGH for multi-service architectures. When a spec defines cross-service interactions, the VE can:

Generate consumer contracts from the spec's interface requirements
Verify providers implement the contract correctly
Detect breaking changes across service boundaries
Critical Insight for VE Design: Pact's consumer-driven model maps directly to the spec-driven VE model:

Spec requirements ≈ Consumer expectations (what the system should do)
Implementation ≈ Provider (what the system actually does)
VE = the entity that defines the contract (from spec) and verifies it against the implementation
Limitation: Requires both consumer and provider to be testable in isolation. Does not help with monolithic applications.

3.3 Property-Based Testing
Tools that define invariants and auto-generate test inputs to find violations.

Hypothesis (Python)
Field	Value
Source	HypothesisWorks/hypothesis
Language	Python
Approach	Strategy-based input generation with shrinking
Capabilities:

Custom strategies for any data type (including complex nested structures)
Automatic shrinking: finds minimal failing example
Stateful testing: model-based tests for stateful systems
Database of interesting examples (persists between runs)
Integrates with pytest
VE Relevance: HIGH for Python libraries and services. The VE agent can:

Define properties from the spec ("this function should always return positive values")
Let Hypothesis find counterexamples automatically
The shrinking capability produces minimal reproducible failures
QuickCheck / fast-check / jqwik
Language	Library	Maturity
Haskell	QuickCheck (original)	Research gold standard
JavaScript/TypeScript	fast-check	Production-ready, excellent docs
Java	jqwik	JUnit 5 integration, property-based + example-based
Go	rapid	Mature, shrinking support
Rust	proptest	Widely used in Rust ecosystem
VE Pattern: Property-based testing is inherently spec-friendly because:

Specs define behavioral properties ("sorting should be idempotent", "API should never return 500")
The VE translates spec requirements into properties
The framework handles test case generation and edge case discovery
Critical Advantage: Properties are HARDER to game than specific test cases. An agent that passes specific tests might fail property-based tests because properties explore the entire input space.

3.4 AI-Assisted Test Generation
Tools that use LLMs to generate tests from code or specifications.

Qodo Cover (formerly CodiumAI Cover Agent)
Field	Value
Source	qodo-ai/qodo-cover
Status	⚠️ Open-source repo no longer maintained (2025-06-15); Pro version via CI
Language	Python, Go, Java, JavaScript (multi-language)
Approach	LLM generates unit tests → validates they increase coverage
How It Works:

Takes source file, existing test file, and coverage report as input
Prompts LLM to generate new test cases targeting uncovered code
Runs tests; only keeps those that actually increase coverage
Iterates until desired coverage or max iterations reached
VE Relevance: MEDIUM — demonstrates the pattern of LLM-generated tests with feedback loop (generate → execute → validate → iterate). However:

Uses code as input (not spec) — this is implementation-driven, not spec-driven
The VE should derive tests from SPEC, not from coverage gaps
The feedback loop pattern (generate → execute → validate) IS relevant
DeepEval
Field	Value
Source	confident-ai/deepeval
Language	Python
Approach	LLM-as-judge evaluation framework
Capabilities:

30+ evaluation metrics (task completion, tool correctness, faithfulness, hallucination)
Agentic metrics: goal accuracy, step efficiency, plan adherence, tool use correctness
MCP metrics: MCP task completion, MCP server usage evaluation
Custom G-Eval (evaluate on any criteria with LLM-as-judge)
DAG-based deterministic evaluation builder
Regression testing across model versions
VE Relevance: HIGH for evaluating AI agent behavior. Key metrics for our use case:

Task Completion → did the implementation agent satisfy the spec?
Tool Correctness → did it use the right tools with right arguments?
G-Eval (custom criteria) → evaluate against any spec-derived criteria
Critical Insight: DeepEval shows that LLM-as-judge can evaluate software against arbitrary criteria specified in natural language. This is exactly what the VE needs — translate spec requirements into evaluation criteria, then judge the implementation.

3.5 Browser & UI Testing
Playwright (+ MCP)
Field	Value
Source	microsoft/playwright
Language	JavaScript, Python, Java, .NET
Browser Support	Chromium, Firefox, WebKit
Key Innovation — Playwright MCP:

AI agents interact with web pages via accessibility tree (not screenshots)
Structured element refs (e5, e10) for deterministic interaction
No vision model required — works with text-only LLMs
Tools: navigation, form filling, assertions, network mocking, storage
VE Relevance: HIGH for any spec that involves web UI behavior. The VE agent can:

Use Playwright MCP to navigate and interact with the application
Assert UI states match spec requirements
Generate end-to-end test scripts from spec scenarios
Run in headless mode in Pool C containers
Uniquely Important: Playwright MCP transforms UI verification from a vision problem to a structured text problem — making it accessible to the VE agent without multi-modal capabilities.

3.6 Mutation Testing
Tools that verify test quality by introducing mutations and checking if tests catch them.

Tool	Language	Approach
PITest	Java	Bytecode-level mutation; 16+ mutators
Stryker	JavaScript/TypeScript	AST-level mutation; comprehensive reporting
mutmut	Python	Source-level mutation; pytest integration
go-mutesting	Go	Source-level mutation with multiple strategies
VE Relevance: HIGH for anti-gaming. Mutation testing answers: "If the implementation had a bug, would our tests catch it?" This directly serves the VE's adversarial role:

VE generates tests from spec
Mutation testing validates that those tests would catch real defects
Mutations that survive = spec requirements insufficiently verified
Design Implication: The VE should run mutation testing as a quality check on its OWN generated tests before declaring verification complete. This is a self-validation loop.

3.7 Container-Based Integration Testing
TestContainers
Field	Value
Source	testcontainers
Languages	Java, .NET, Go, Python, Node.js, Rust
Approach	Programmatic Docker container management for tests
Capabilities:

Spin up databases, message brokers, web servers as disposable test dependencies
Language-native API (no YAML, no external orchestration)
Modules for 60+ services (PostgreSQL, Kafka, Redis, Elasticsearch, etc.)
Lifecycle management: start before test, clean up after
VE Relevance: HIGH for integration verification. When a spec requires interaction with external dependencies (database, queue, cache), the VE can:

Spin up the required dependency as a container
Run the implementation against a real (but disposable) instance
Verify behavior against spec requirements in a realistic environment
3.8 Architecture & Constraint Testing
ArchUnit (Java) / archunit-ts / dep-tree
Tool	Language	Purpose
ArchUnit	Java	Architecture rules as code; package dependencies, layer violations
archunit-ts	TypeScript	Same concept for TypeScript projects
dep-tree	Go	Dependency visualization and constraint checking
VE Relevance: MEDIUM — useful when specs define architectural constraints ("this module should not depend on X", "all controllers must go through the service layer"). The VE can verify architecture compliance without running the application.

Policy-as-Code (OPA / Conftest / Kyverno)
Tool	Scope
OPA/Rego	General-purpose policy evaluation
Conftest	Validate structured data (YAML, JSON, HCL, Dockerfile)
Kyverno	Kubernetes policy enforcement
VE Relevance: MEDIUM-HIGH for infrastructure and configuration verification. When specs define infrastructure requirements, the VE can express them as policies and validate configurations.

3.9 Performance & Load Testing
Tool	Language	Approach
k6	Go (JS scripting)	Developer-centric load testing; CI/CD-friendly
Locust	Python	Distributed load testing with code-based test definitions
Artillery	JavaScript	YAML-defined load test scenarios
Gatling	Scala/Java	High-performance simulation framework
VE Relevance: MEDIUM — needed when specs include non-functional requirements (response time, throughput, concurrent users). The VE translates NFR spec clauses into load test configurations.

3.10 Security Testing
Category	Tools	Approach
SAST	CodeQL, Semgrep, SonarQube	Static analysis of source code
DAST	OWASP ZAP, Burp Suite	Dynamic testing of running applications
SCA	Snyk, Dependabot, OSV-Scanner	Dependency vulnerability scanning
Secret Detection	TruffleHog, GitLeaks	Credential leak detection in code/history
Fuzzing	libFuzzer, AFL++, Jazzer	Crash/vulnerability discovery via random input
VE Relevance: MEDIUM — security scanning is more of a governance concern (handled by Governance Engine gates) than a spec verification concern. However, when specs explicitly define security requirements ("must not expose PII", "must use TLS"), the VE should verify compliance.

4. Emerging Patterns — How Industry Approaches Generic Verification
4.1 The "Agent + Toolkit" Pattern
Observation: No tool in the landscape solves generic verification alone. The emerging pattern is:

┌─────────────────────────────────────────────────┐
│              VE Agent (LLM-powered)              │
│                                                  │
│  1. Read spec → extract verifiable requirements  │
│  2. Analyze target → identify interface type     │
│  3. Select strategy → choose appropriate tools   │
│  4. Generate tests → produce framework-specific  │
│  5. Execute → run in isolated container          │
│  6. Judge → LLM-as-judge for semantic checks     │
│  7. Report → standardized verdict schema         │
└───────────┬─────────────────────────────────────┘
            │ selects from
            ▼
┌─────────────────────────────────────────────────┐
│            Verification Toolkit                   │
├─────────────────────────────────────────────────┤
│ Schema:   Schemathesis, Step CI                  │
│ Contract: Pact                                   │
│ Property: Hypothesis, fast-check, jqwik, rapid   │
│ Browser:  Playwright MCP                         │
│ Infra:    Conftest, OPA                          │
│ Mutation: PITest, Stryker, mutmut                │
│ Load:     k6, Locust                             │
│ Security: CodeQL, Semgrep, ZAP                   │
│ Integration: TestContainers                      │
│ Generic:  pytest, JUnit, go test, Vitest         │
│ Judge:    DeepEval (LLM-as-judge)                │
└─────────────────────────────────────────────────┘
4.2 The "Spec as Contract" Pattern
Multiple tools converge on the idea that specifications ARE contracts:

Tool	"Spec" Format	"Contract" Becomes
Schemathesis	OpenAPI schema	Auto-generated fuzz tests
Pact	Consumer expectations	Provider verification suite
Hypothesis	Property definitions	Generated test inputs
Conftest	Policy files	Configuration validators
DeepEval	Natural language criteria	LLM-as-judge evaluations
Our ISO spec.md plays this role: it defines what the implementation must do, and the VE translates it into executable verification — the same pattern, at a higher abstraction level.

4.3 The "Three Verification Tiers" Pattern
Emerging from the landscape, three distinct verification tiers appear:

Tier	Type	Speed	Confidence	Tools
T1: Deterministic	Schema validation, type checking, linting, policy checks	Fast (seconds)	High (binary pass/fail)	Schemathesis, Conftest, ArchUnit
T2: Generative	Property-based, contract, mutation, fuzz testing	Medium (minutes)	High (statistical)	Hypothesis, Pact, PITest, k6
T3: Semantic	LLM-as-judge, behavioral assessment, spec compliance	Slow (tens of seconds per evaluation)	Medium (probabilistic)	DeepEval, custom G-Eval criteria
Design Implication: The VE should run all three tiers, with T1 providing fast feedback, T2 providing thorough coverage, and T3 providing spec-aligned semantic judgment. T1+T2 are "hard" verdicts (deterministic); T3 is "soft" (requires confidence thresholds).

4.4 The "Universal Test Execution" Pattern
Several tools converge on abstracting test execution from test definition:

Tool	Abstraction
TestContainers	"Give me a Postgres" → containerized DB available
SWE-ReX	"Run this in a sandbox" → isolated shell environment
Playwright MCP	"Click this button" → structured browser interaction
Step CI	"Test this workflow" → YAML → execution across protocols
Design Implication: The VE doesn't need to know HOW to run a PostgreSQL instance — it needs to declare WHAT it needs and let the substrate provide it. This is the container-as-dependency pattern that TestContainers pioneered.

4.5 The "Feedback Loop" Pattern (from Qodo Cover)
Generate tests from spec
        ↓
Execute tests against implementation
        ↓
Analyze results (pass/fail, coverage, mutations killed)
        ↓
If insufficient → regenerate with more targeted tests
        ↓
Repeat until confidence threshold met or iteration budget exhausted
This is structurally identical to DFT's retry/iteration model for implementation — but applied to verification. The VE should iterate on test generation just as the implementation agent iterates on code.

5. Proposed VE Architecture — The Adaptive Verification Agent
Based on the landscape survey, the VE layer should be designed as an adaptive agent that composes verification strategies rather than a monolithic verification tool.

5.1 Interface Detection Phase
Before generating any tests, the VE must identify the target's interface type:

interface_detection:
  inputs:
    - spec.md (required — defines WHAT to verify)
    - source code (read-only — for interface discovery)
    - project manifest (package.json, go.mod, pom.xml, etc.)
    - API schemas (openapi.yaml, .proto, schema.graphql)
    - README / docs (secondary context)
  
  outputs:
    - interface_map:
        - type: rest_api
          schema: openapi.yaml
          endpoints: [/users, /orders, ...]
        - type: library
          language: python
          entry_points: [mylib.process, mylib.validate]
        - type: web_ui
          framework: react
          routes: [/dashboard, /settings]
5.2 Strategy Selection Phase
Based on interface type and spec requirements, select verification strategies:

Spec Requirement Type	Interface	Strategy	Tool
"API returns X for input Y"	REST	Schema-driven + specific	Schemathesis + pytest
"Service handles concurrent requests"	REST/gRPC	Load + property	k6 + Hypothesis
"Function never returns negative"	Library	Property-based	Hypothesis/fast-check
"UI shows error on invalid input"	Web UI	Browser automation	Playwright MCP
"Service A calls Service B correctly"	Event/HTTP	Contract	Pact
"Module cannot depend on X"	Any	Architecture constraint	ArchUnit/Conftest
"Must handle 1000 req/s"	REST/gRPC	Performance	k6/Locust
"No SQL injection possible"	REST/Library	Security + fuzz	Semgrep + Schemathesis
5.3 Test Generation Phase
The VE agent generates tests appropriate to the selected strategy:

For each verifiable requirement in spec:
  1. Map requirement → interface type
  2. Select primary strategy (deterministic if possible, semantic if needed)
  3. Generate test code in the project's native test framework
  4. Add property-based tests for invariant requirements
  5. Add LLM-as-judge evaluations for subjective/complex requirements
  6. Tag tests: {published | hidden} based on anti-gaming classification
5.4 Execution Phase
Tests execute in Pool C containers with appropriate substrate:

┌─────────────────────────────────────────────────────┐
│ Pool C Container (Kata isolation)                     │
├─────────────────────────────────────────────────────┤
│ ┌─────────────┐ ┌──────────────┐ ┌──────────────┐  │
│ │ App Under   │ │ Test Deps    │ │ VE Test      │  │
│ │ Test (AUT)  │ │ (TestContain)│ │ Suite        │  │
│ └─────────────┘ └──────────────┘ └──────────────┘  │
│         ↓                ↓                ↓         │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Test Runner (pytest/JUnit/go test/Vitest)       │ │
│ └─────────────────────────────────────────────────┘ │
│                          ↓                           │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Verdict Collector → Verdict Service             │ │
│ └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
5.5 Judgment Phase (T3 — Semantic)
For requirements that cannot be deterministically verified:

semantic_evaluation:
  framework: deepeval_pattern  # LLM-as-judge
  criteria_source: spec.md
  
  example_criteria:
    - "Error messages should be user-friendly and actionable"
    - "Code should follow project conventions"
    - "Documentation accurately describes behavior"
  
  judgment_model: separate_from_implementation_model  # independence
  confidence_threshold: 0.85
  consensus_required: 2_of_3  # multiple judge passes for soft verdicts
6. Language Ecosystem Mapping
For the VE to be genuinely language-agnostic, it needs adapter knowledge per ecosystem:

Language	Test Framework	Property Testing	Coverage	Mutation	Package Manifest
Python	pytest	Hypothesis	pytest-cov (Cobertura)	mutmut	pyproject.toml, requirements.txt
JavaScript/TypeScript	Vitest, Jest	fast-check	v8/istanbul	Stryker	package.json
Go	go test	rapid	go tool cover	go-mutesting	go.mod
Java	JUnit 5	jqwik	JaCoCo	PITest	pom.xml, build.gradle
C#	xUnit, NUnit	FsCheck	Coverlet	Stryker.NET	*.csproj
Rust	cargo test	proptest	cargo-llvm-cov	cargo-mutants	Cargo.toml
C/C++	GoogleTest, Catch2	RapidCheck	gcov/llvm-cov	mull	CMakeLists.txt
DFT Heritage: DFT's feature-mapping.yaml and entrypoint.sh already contain language-bootstrap knowledge for Python, Go, JavaScript, C, and C++. This maps directly to the VE's need to set up the correct test execution environment per language.

7. Anti-Gaming Mechanisms
The VE's adversarial role requires mechanisms that prevent implementation agents from gaming the verification:

7.1 Hidden Evaluation Sets
VE generates tests that are NEVER shown to implementation agents
Stored separately from published test suites
Only revealed in Phase 9 (full verification) results
QCOE audits hidden eval quality periodically
7.2 Property-Based Tests (Inherently Hard to Game)
Properties define WHAT must hold, not specific input/output pairs
Hypothesis/fast-check explore the full input space
Agent cannot "memorize" specific test cases because they're generated randomly
Shrinking finds minimal failures — hard to have a "just barely passing" implementation
7.3 Mutation Testing Validation
VE runs mutation testing on its OWN test suite
If mutations survive → tests are insufficient → regenerate
Measures "would these tests actually catch bugs?" not just "do they pass?"
7.4 Semantic Variation
LLM-as-judge evaluations use different phrasings each run
Criteria extracted from spec but expressed differently to prevent prompt-gaming
Multiple independent judges (consensus model)
8. Cross-References
Document	Section	Relationship
architecture.md	§7 VE Isolation	Pool C separation, informational isolation
integration.md	§8–9 Verification Pipeline	Verdict Service modes, continuous sidecar
requirements.md	VHIR-001–011	Verification infrastructure requirements
ADR-004	§2 DFT Inventory	Language bootstrap knowledge, container lifecycle
evals-observability.md	Trace grading	Grader taxonomy (code-based, model-based, human)
foundations.md	§3 Three-Agent Model	Planner → Generator → Evaluator pattern
runtimes-implementations.md	SWE-ReX	Execution substrate abstraction
agentic-evaluation.md	§6.3 VE Independence	Informational isolation design
constraints-guardrails.md	OS-level isolation	Pool C security boundaries
9. Open Questions for Session 5 Design
Toolkit pre-installation vs. on-demand: Should Pool C containers come pre-loaded with all verification tools, or should the VE agent install what it needs? (Trade-off: startup time vs. container size)
Test framework selection authority: Does the VE choose the test framework, or must it match the project's existing test setup? (Trade-off: VE independence vs. project conventions)
Semantic judgment calibration: How do we calibrate LLM-as-judge confidence thresholds? What's the "sufficient" confidence for a pass verdict?
Cross-service verification: When a spec spans multiple services, who orchestrates the multi-service verification environment? (VE agent? L2 orchestrator? TestContainers composition?)
Feedback to implementation: How much verification failure detail flows back to the implementation agent? (Too little = unhelpful; too much = enables gaming)
VE iteration budget: How many generate→execute→judge cycles does the VE get before declaring a verdict? (Analogous to DFT's implementation retry budget)
End of landscape survey. Key finding: the verification layer is NOT a single tool but an adaptive AI agent composing from a toolkit of deterministic (T1), generative (T2), and semantic (T3) verification strategies — selected based on the target program's interface type and the spec's verifiable requirements.


GitHub - schemathesis/schemathesis: Catch API bugs before your users do

Catch API bugs before your users do. Contribute to schemathesis/schemathesis development by creating an account on GitHub.

github.com
Agentic — Deep Evaluation (Internal Framewo... by Ferreyra, Marcos
Yesterday 7:57 PM
Ferreyra, Marcos

Agentic — Deep Evaluation (Internal Framework)
Field	Value
Parent	Landscape Overview
Source	ncrvoyix-swt-cfr/agentic (internal repository)
Status	Revised (recalibrated post-ADR-004; source-verified April 2026)
Provenance	Internal — built by NCR Voyix SWT CFR team
1. Overview & Provenance
Agentic is a production-grade AI workflow orchestration engine built on .NET and Aspire. It coordinates AI agents (GitHub Copilot CLI, Copilot SDK, Claude Code, and extensible providers) through durable, multi-cycle workflows that implement, test, review, fix, and verify code changes autonomously.

Key Statistics
Metric	Value
Source projects	9 (.NET solution)
Test projects	6
Built-in plans	36 across 7 categories
Agent profiles	14 (5 general + 9 SDET specialist)
Specs completed	157+ (each spec = one feature delivered)
Execution surfaces	4 (CLI, Web, Desktop/MAUI, GitHub Actions)
Agent providers	3 (Copilot CLI, Copilot SDK, Claude Code)
Why This Evaluation Matters
Agentic is developed internally within our organization. Unlike external tools evaluated in the runtimes supplement, Agentic represents a build option — we can adopt, extend, or learn from it directly. Its 157+ completed specs demonstrate production maturity that no external tool in our landscape survey has matched. The evaluation must be fair and balanced: acknowledging Agentic's strengths while identifying where our ISO harness design adds value that Agentic does not provide.

2. Architecture Analysis
Project Structure
Project	Role	Relevance to ISO Harness
Agentic.Engine.Core	Core orchestration logic, step handlers, agent providers, RAG subsystem	Maps to Orchestrator Engine + Agent Gateway + Context Assembler
Agentic.Engine	ASP.NET Core API server, SignalR hub, EF Core SQLite	Maps to State Store + Observability Plane
Agentic.Cli	Console application (agentic command)	Execution entry point — no ISO equivalent (our harness is API-first)
Agentic.Web	Blazor dashboard for monitoring	Maps to Observability Plane (visualization layer)
Agentic.Maui	Native desktop app wrapper	No ISO equivalent — we don't plan a desktop client
Agentic.UI	Shared Razor component library	No ISO equivalent
Agentic.Shared	DTOs, contracts, models	Maps to our interface contracts
Agentic.AppHost	Aspire host (orchestrates Engine + Web)	Maps to our deployment architecture
Agentic.ServiceDefaults	OpenTelemetry, health checks	Maps to Observability Plane infrastructure
Data Flow
Agentic's execution model follows a clear pipeline:

User → API → OrchestrationEngine → OrchestrationWorker → StepDependencyGraph (DAG)
  → StepExecutionEngine → Step Handler → AgentProviderRegistry → Agent (CLI/SDK)
  → Output parsing → SQLite persistence → SignalR broadcast → Web UI
Key design choices:

Plan-as-Code: Workflows are declarative YAML — version-controlled, reviewable, composable
Cycle-based iteration: Plans execute in cycles (implement → assess → decide → continue) with diminishing-returns detection
Wave-based parallel execution: DAG scheduler dispatches ready steps in parallel waves (up to 20 concurrent steps via semaphore)
Checkpoint persistence: SQLite-backed checkpoints enable crash recovery and resume
Multi-provider: Agent invocations route through a provider registry — CLI process spawn (Copilot, Claude) or in-process SDK calls
Extension Model
Agentic is extensible at four primary points:

Plans — YAML definitions with typed parameters, conditions, outputs
Agent Profiles — Per-step persona and model configuration
Step Types — New step types via StepExecutionEngine dispatch
Agent Providers — IAgentProvider interface for new AI backends
This is a plugin model, not a framework model — extensions add capabilities without modifying core behavior.

Plan Composability (Source-Code Verified)
Agentic's plan composition model is more mature than the initial evaluation captured. Source-level analysis (April 2026) reveals a fragment-based composition system that enables reusable, parameterized workflow building blocks:

Fragment Architecture
Plans compose via an imports: declaration that resolves reusable fragments (.agentic/fragments/*.fragment.yaml) at parse time:

Plan (ParsedPlan)
  ├── imports: List<ImportDeclaration>
  │   └── fragment: ──→ FragmentDefinition (resolved from disk)
  │         ├── parameters: Dictionary<string, PlanParameter>
  │         └── steps: List<PlanStep>
  └── steps: List<PlanStep>  ← expanded to include inlined fragment steps
Composition pipeline (from specs/110-plan-composition):

Deserialize plan YAML to ParsedPlan
ResolveImports() — Fragment resolution, circular dependency detection, parameter binding
Apply template expansion (runs AFTER import resolution)
Validate steps, refs, DAG
Return expanded ParsedPlan indistinguishable from monolithic plan
Fragment capabilities:

Typed parameters — string, int, bool, enum with allowedValues, required/optional, defaults
Parameter passing — with: blocks bind caller values to fragment parameters
Per-step overrides — Importing plans can customize fragment behavior without modifying the fragment file
Multi-import with aliasing — Same fragment imported multiple times using as: alias with automatic step ID prefixing
Circular dependency detection — Build-time validation prevents import cycles
Architectural characteristic: parse-time, not runtime. Fragments are inlined before execution — the engine sees a flat, expanded plan. This means:

✅ Composition is deterministic and validatable (dry-run shows expanded plan with fragment provenance)
✅ No runtime overhead for composition
❌ Cannot dynamically compose workflows based on runtime discovery (e.g., L1 cannot conditionally import fragments based on per-repo analysis results)
ISO harness implication: The fragment model validates plan-as-code as a mature pattern for defining reusable workflow stages. Our L2 pipeline stages (SPECIFY → PLAN → TASKS → IMPLEMENT → REVIEW → MERGE) could be expressed as composable YAML definitions — but executed by the governance-aware L2 state machine, not by Agentic's flexible execution engine. The fragment model is a reference architecture for authoring workflows; the L2 state machine remains the enforcement engine. See DD-10 in integration.md.

Execution Pluggability Clarification (Source-Code Verified)
The initial evaluation noted Agentic has "4 execution surfaces" and an extensible provider model. Source analysis reveals an important architectural distinction that affects our Session 5 execution environment design:

What IAgentProvider Actually Abstracts
// src/Agentic.Shared/Contracts/IAgentProvider.cs
public interface IAgentProvider
{
    string Name { get; }
    Task<AgentResult> ExecuteAsync(AgentRequest request, ...);
    Task<HealthCheckResult> CheckHealthAsync(...);
    Task<string> LoadPromptAsync(string? prompt, string? promptFile, ...);
}
This interface abstracts the AI agent backend — which LLM/tool processes the step (Copilot CLI, Copilot SDK, Claude Code, etc.). Plans can override the provider per step via a provider: field. The AgentProviderRegistry selects the active provider at runtime.

What this IS: Agent backend pluggability — swap AI models/tools without changing plans or orchestration logic.

What this is NOT: Execution substrate pluggability. The IAgentProvider interface says nothing about:

Container runtime (Docker, Podman, gVisor, Kata)
Infrastructure provisioning (local, K8s, cloud VM)
Network isolation policy (host, allowlist, none)
Workspace storage (local SSD, NFS, EBS)
These are separate concerns addressed by Agentic's SandboxProcessService (Docker/Podman single-tier) and by our ADR-001 (tiered hybrid: runc/gVisor/Kata).

Three pluggability dimensions (clarified for Session 5):

Dimension	What It Swaps	Agentic Abstraction	ISO Harness Abstraction
Agent backend	Which AI model/tool processes the step	IAgentProvider interface ✅	Agent Gateway pool registry
Runtime substrate	Which container/VM runs the workspace	SandboxProcessService (single-tier)	ADR-001 tiered hybrid (per-pool)
Orchestration engine	Which coordinator drives the workflow	Fixed (OrchestrationEngine)	Per-layer: L1 coordinator, L2 state machine, L3 DFT engine
ISO harness implication: Our architecture must separate these three dimensions explicitly. Agentic cleanly solves dimension 1 (agent backend). Our ADR-001 solves dimension 2 (runtime substrate). Dimension 3 (orchestration per layer) is addressed by ADR-004's layer boundary contracts. No single abstraction covers all three — and attempting to unify them would conflate concerns that have different lifecycle, ownership, and scaling characteristics.

GitHub Actions clarification: Agentic has 4 deployment surfaces — CLI, Web UI, Desktop (MAUI), and GitHub Actions. GitHub Actions is one CI/CD trigger surface, not the only execution runtime. For ISO Lane 2, the execution substrate is Docker/K8s (ADR-001), independent of how workflows are triggered. GitHub Actions could serve as the CI trigger for Lane 1 Express workflows, but the execution environment for ISO-governed work is determined by pool-level substrate policy, not by the deployment surface.

3. Capability Mapping — Agentic vs ISO Harness
This section maps Agentic's capabilities against the functional areas defined in our ISO harness architecture. Note: the ISO harness is a design — these are specified capabilities, not implemented ones. Agentic is production software with 157+ specs delivered. The comparison reflects this asymmetry by scoring on two axes.

Capability Coverage
The table below uses two ratings per area:

Operational Maturity — How production-ready is Agentic's implementation today?
ISO Fit — How well does Agentic's approach align with our target architecture's requirements?
#	Capability Area	Agentic Equivalent	Operational Maturity	ISO Fit	Notes
1	Orchestration	OrchestrationEngine + CycleRunner + StepExecutionEngine + StepDependencyGraph	HIGH	HIGH	Durable execution, DAG scheduling, cycle management, diminishing-returns detection. Plans ≈ L2 equivalent. Steps ≈ L3 equivalent. No explicit L1 (cross-repo) — propagate-changes plan is a workaround, not a coordinator.
2	Context Management	ContextWindowingService + context files + RAG auto-injection + MemoryService	HIGH	MEDIUM	Operationally mature: 4 truncation strategies, model-aware budgets, RAG with vector store and multi-embedding, cross-run memory. But Agentic's context is windowed (truncation-based), not assembled (structured-format-based). No 3-layer instruction model, no per-stage specification.
3	Agent Routing	AgentProviderRegistry + CopilotSdkClientManager + SessionGroupManager	HIGH	MEDIUM	Production-hardened: 3 providers, session pooling with heartbeat, 10-category error classification, credit-saving session groups. But no tiered routing cascade, no per-pool namespace isolation, no output compression.
4	State Persistence	SQLite via EF Core + checkpoint persistence	HIGH	HIGH	Full crash recovery, checkpoint resume, stale-step watchdog, data retention policies. Our State Store concept maps closely, though our design specifies git-centric state (every transition = git commit) while Agentic uses SQLite-centric state.
5	Evaluation	Eval trace replay + regression detection + assessment steps	MEDIUM	LOW	Eval replay is innovative: capture, override, compare, detect regression. But evaluations are self-assessments — no independent verification pipeline, no multi-tier grading, no hidden eval model. This is the largest philosophical gap.
6	Traceability	—	NONE	NONE	No requirement-to-verifier traceability. No spec-to-test mapping. Agentic operates at the plan/step level, not the requirement level.
7	Governance	Threat detection + checkpoint approvals + onFailure recovery	MEDIUM	LOW	Threat detection is strong: CodeQL + Semgrep + OWASP ZAP with dedup. Checkpoint gates support human approval. onFailure handlers provide advisory error recovery. But governance is opt-in per plan, not system-wide. No QCOE release authority, no mandatory gate sequencing, no blocking enforcement hooks (see §4.6 correction).
8	Merge Control	Git operations within plan steps	MEDIUM	LOW	Plans commit, push, and create PRs. But merge is an agent action, not a governance-controlled step. No spec archival, no merge-condition enforcement.
9	Execution Isolation	SandboxProcessService (Docker/Podman)	MEDIUM	MEDIUM	Container-per-step with network modes, resource limits, bind mounts. Production-proven. But single tier (Docker or Podman), no gVisor/Kata progression, no inner-sandbox network proxy.
10	Observability	OpenTelemetry + Aspire + ActivityLogService + cost tracking	HIGH	HIGH	Distributed traces, structured metrics/logs, per-step token tracking, cost reporting, real-time UI, health checks. This is the most production-mature area.
Summary
Rating	Operational Maturity (today)	ISO Fit (target alignment)
HIGH	5 areas (orchestration, context, routing, state, observability)	3 areas (orchestration, state, observability)
MEDIUM	4 areas (evaluation, governance, merge, execution)	3 areas (context, routing, execution)
LOW	—	3 areas (evaluation, governance, merge)
NONE	1 area (traceability)	1 area (traceability)
Key insight: Agentic is operationally mature in most areas (9/10 MEDIUM or above). The ISO Fit gaps cluster around governance, evaluation independence, and traceability — which are precisely our design's differentiating concerns.

4. Design Pattern Comparison
4.1 Workflow Orchestration
Aspect	Agentic	ISO Harness
Primary abstraction	Plan (YAML) with typed steps	L1/L2/L3 nesting with state machines
Orchestration model	Flat plan → step DAG → parallel waves	Hierarchical: L1 (cross-repo) → L2 (spec-unit pipeline) → L3 (agentic task loop)
Cycle management	Configurable cycles with diminishing-returns detection	Fixed stage sequence (SPECIFY → PLAN → TASKS → IMPLEMENT → REVIEW) with iteration within IMPLEMENT
DAG scheduling	StepDependencyGraph with Kahn's algorithm, wave dispatch	WBS-driven DAG scheduling with batch-level parallelism
Cross-repo	propagate-changes plan (workaround)	Dedicated L1 coordinator
Spec integration	.specify/ directory (Spec-Kit compatible)	Spec-Kit is the foundational workflow driver
Analysis: Agentic's plan model is more flexible (any workflow can be expressed in YAML) but less structured (no enforced stage sequence). Our L2 state machine enforces the ISO methodology's I→S→O flow, which guarantees that every change goes through specify→plan→implement→review→verify. Agentic lets you skip stages — which is both a strength (flexibility for hotfixes) and a weakness (no methodology enforcement).

The dev-orchestration-v1 plan is the closest Agentic equivalent to our L2 pipeline: analyze → specify → plan → tasks → implement → review → merge. But it's one of 36 plans — a convention, not an architectural constraint.

Fragment composition update (source-verified): Agentic's fragment-based composition model (§2 Plan Composability) adds a significant reusability dimension not captured in the comparison above. Fragment imports with typed parameters, multi-import aliasing, and per-step overrides enable building complex workflows from tested, version-controlled building blocks. This validates the plan-as-code direction for workflow authoring. However, fragments are resolved at parse time (not runtime), which means the composition is static — the plan structure must be known before execution begins. This is sufficient for L2/L3 (where the stage sequence is fixed) but insufficient for L1 (where cross-repo coordination may depend on per-repo discovery results).

4.2 Context Management
Aspect	Agentic	ISO Harness
Context strategy	ContextWindowingService with 4 truncation strategies	Context Assembler with 3-layer instruction model (DA-01)
Token awareness	Model-aware byte budgets (e.g., Claude 600KB, GPT 400KB)	Per-stage token budgets with per-component caps (DA-06)
Overflow handling	Truncation (tail, head-tail, errors-only, smart-truncate)	Priority-based overflow: truncate lowest-priority components first (DA-06)
RAG integration	Built-in: vector store, multi-embedding, language-aware chunking	Not in current design — identified as a gap
Cross-run memory	MemoryService with confidence scoring and age-based pruning	Not in current design
Session groups	Cross-step session reuse for credit savings	Fresh conversation per stage (DFT model) — explicitly different design philosophy
Analysis: Agentic's context management is more operationally mature (RAG, memory, session groups) but less architecturally principled (no instruction layering, no structured format). Our design prioritizes deterministic, clean-context assembly; Agentic prioritizes practical credit efficiency. These are complementary, not contradictory — our instruction model (DA-01) could sit atop Agentic's windowing service.

The session groups pattern is interesting but conflicts with our DFT model's "fresh conversation per stage" principle (V-02). Agentic uses session continuity for cost savings; we use conversation isolation for quality. Both have evidence supporting their approach — this is a genuine design tension.

4.3 Execution Isolation
Aspect	Agentic	ISO Harness
Substrate	Docker or Podman (single tier)	Tiered: runc → gVisor → Kata (per pool trust level)
Isolation model	Container-per-step with bind-mounted workspace	Container-per-pool with hot workspace persistence
Network isolation	Host or none (binary choice)	Proxy-based allowlist filtering (DA-08)
Lifecycle	Fresh container per step (--rm flag)	Hot workspace across stages within same pipeline
Analysis: Agentic's container-per-step model is simpler and more isolated (each step gets a clean container). Our hot-workspace model preserves state across stages for efficiency (no re-clone, no re-install). Agentic's approach avoids workspace contamination between steps; ours avoids setup overhead. Our tiered substrate (gVisor/Kata for untrusted workloads) adds a security dimension Agentic doesn't address.

4.4 Agent Routing
Aspect	Agentic	ISO Harness
Provider model	AgentProviderRegistry with 3 providers	Agent Gateway with pool-based model allocation
Model selection	Per-step model/profile override	Per-pool model rotation with tier-based selection
Session management	CopilotSdkClientManager with LRU pool + heartbeat	Fresh invocation per stage (no session pooling)
Routing optimization	None — all requests go to model	DA-12: 4-tier cascade (pattern → state → keyword → model)
Error handling	SdkErrorClassifier with 10 categories + remediation	Not yet specified at this level of detail
Analysis: Agentic's error classification and SDK session management are production-hardened — 10 error categories with user-friendly remediation, heartbeat monitoring, connection loss detection. Our design lacks this operational detail. Their provider registry is extensible (IAgentProvider interface). Our tiered routing cascade (DA-12) would be a valuable addition to Agentic's architecture.

4.5 Evaluation
Aspect	Agentic	ISO Harness
Eval model	Eval trace capture → replay with overrides → regression detection	Verdict Service with multi-tier grading (DA-14)
Independence	Self-assessment (dev agent evaluates own work)	Independent verification (Pool C agents evaluate Pool A/B output)
Regression	Dataset-wide automated regression checking with score thresholds	Infrastructure noise-aware quality gates (DA-15)
Grading	Score-based comparison (IMPROVED/UNCHANGED/REGRESSED)	Multi-tier: deterministic → model-rubric → trajectory-critic (DA-14)
Analysis: Agentic's eval framework is operationally mature (capture, replay, compare, CI integration). But it lacks the independence guarantee that is central to our Verdict Service — Agentic agents assess their own work. Our triple-pipeline isolation ensures that verification is performed by agents that never saw the implementation, using different model configurations. This is our strongest differentiator.

4.6 Governance
Aspect	Agentic	ISO Harness
Security scanning	Threat detection: CodeQL + Semgrep + OWASP ZAP with cross-scanner dedup	Not yet designed at this detail (DA-10 covers input analysis)
Approval gates	Checkpoint steps with timeout policies (auto-approve/reject/cancel)	13 governance gates with PE/VE/QCOE authority
Enforcement model	Opt-in per plan (security.threatDetection: true)	System-wide, mandatory, tiered (advisory prompt + enforced hooks per DA-09)
Release authority	No concept	QCOE release block — independent body with release/hold authority
Hook system	onFailure recovery handler (advisory — see §10)	CONSTITUTION.md pre/post-execution hooks (DA-09)
Analysis: Agentic's threat detection pipeline is more mature than anything in our current design — three scanners with intelligent deduplication, severity thresholds, and allow-lists. But governance in Agentic is opt-in and plan-scoped, while our design is mandatory and system-wide. Our Governance Engine with QCOE release authority provides organizational-level assurance; Agentic provides tool-level security scanning. These serve different purposes and are complementary.

Hook correction (source-verified): The original evaluation described Agentic's hook system as onStepStart/onStepEnd/onCycleEnd lifecycle hooks. Source code analysis (April 2026, spec 064-step-onfailure-handler) reveals that only onFailure exists — a recovery handler that runs after all retry attempts are exhausted. Key limitations: (1) advisory only — handler failure does not override step failure, (2) synchronous but cannot abort/skip/retry the parent workflow, (3) does not fire on cancellation, (4) no onSuccess, onStepStart, or onCycleEnd equivalents exist. This means Agentic's hook system is suitable for error recovery and cleanup but insufficient for governance gate enforcement. Our DA-09 blocking pre/post-execution hooks remain architecturally necessary.

5. What Agentic Solves That We Planned to Build
Design Actions Already Addressed
DA	Our Recommendation	Agentic Status	Notes
DA-05	Structured context format	Partially addressed	Context windowing exists but no XML-tagged structured format. Agentic uses plain text with truncation, not structured assembly.
DA-06	Token budgeting per stage	Partially addressed	Model-aware byte budgets exist (e.g., Claude 600KB). But no per-component caps or priority-based overflow.
DA-07	Proactive backpressure	Not addressed	No output compression at gateway level. Tool outputs flow directly into context.
DA-11	SWE-ReX for substrate	Alternative approach	Agentic uses SandboxProcessService with Docker/Podman — a simpler but proven approach.
DA-12	Tiered routing cascade	Not addressed	All requests go directly to model provider. No zero-token resolution tiers.
DA-13	Per-call cost ledger	Fully addressed	Per-step token tracking with per-model breakdown, aggregated cost reporting, budget controls. Production-grade.
DA-14	Trace-based grader taxonomy	Partially addressed	Eval trace replay with score comparison exists. But no multi-tier grading (deterministic/model-rubric/trajectory-critic).
Capabilities Beyond Our Current Design
Capability	Agentic Implementation	Our Design Gap
Executable plan library with fragment composition	36 built-in YAML plans across 7 categories, composable via fragment imports with typed parameters, multi-import aliasing, and per-step overrides (§2 Plan Composability)	We have design documents, not executable workflows. No plan-as-code model. Fragment composition validates reusable workflow authoring for L2/L3 stages.
RAG subsystem	Vector store, multi-embedding (code + text), language-aware chunking for 10 languages, cross-run memory	Not in our design. RAG could significantly improve Context Assembler quality.
Session groups	Cross-step session reuse with what_to_do_next tool pattern for credit savings	Conflicts with DFT "fresh conversation" model but offers significant cost reduction.
Multi-surface execution	CLI + Web UI + Desktop + GitHub Actions — same engine, all surfaces	We specify API-first but no UI/CLI. Agentic's multi-surface model is production-ready.
Agent error classification	10-category SdkErrorClassifier with remediation steps, transient/non-transient classification	We haven't designed error handling at this granularity.
Threat detection pipeline	CodeQL + Semgrep + OWASP ZAP with dedup, severity thresholds, allow-lists	Our DA-10 (input analysis) is narrower. Agentic's pipeline is broader and more mature.
Eval trace replay	Capture baseline → replay with overrides → compare → detect regression	Our Verdict Service design doesn't include replay/comparison capability.
Cross-run memory	MemoryService with confidence scoring, tag-based categorization, age-based pruning	Not in our design. Valuable for agents to learn from past runs.
Diminishing-returns detection	Automatic convergence evaluation across cycles	Our stall detection (integration.md §3.5) is simpler — consecutive failure count only.
Production Readiness and Organizational Assets
Agentic demonstrates production-grade engineering in several areas our design has only specified abstractly. These are not just technical features — they represent organizational readiness that would take significant effort to replicate:

Crash recovery: Checkpoint-based persistence with automatic resume — not just "the design supports it" but tested across 157+ specs
Health monitoring: Database, coordinator, agent CLI, settings, and workspace health checks with a stale-step watchdog
Drain mode: Graceful shutdown that lets active steps complete — critical for 24/7 operation
Data retention: Configurable cleanup policies for old run data
CI integration: Composite GitHub Action with input/output contracts — production-deployable today
36 executable plans and 14 agent profiles: A ready-to-use library that covers development, code review, test generation, CI analysis, security audit, and more. Our design has zero executable assets.
4 deployment surfaces: CLI + Web UI + Desktop (MAUI) + GitHub Actions — the same engine runs everywhere. Our design specifies API-first but has no UI or CLI.
Drain mode, stale-step watchdog, health checks, retention policies: Operational maturity patterns that took significant engineering effort to harden.
These assets materially strengthen the case for Option A (adopt Agentic). Even under Option C (hybrid), teams can deploy Agentic today for non-ISO workflows while the ISO harness is being built.

6. Where Our Design Adds Value Beyond Agentic
Important caveat: The ISO harness is a design — these capabilities are specified, not yet implemented. Agentic is production software. This section describes the architectural intent of our design and where, if implemented as specified, it would address gaps that Agentic does not.

6.1 Triple-Pipeline Isolation
Agentic runs all agent work in a single pipeline. A review step may follow an implementation step, but there is no architectural barrier preventing context leakage between them.

Our design specifies three isolated pipelines:

Pool A (Dev): Implementation agents with workspace access
Pool B (Test): Testing agents with implementation-blind context
Pool C (Review): Verification agents that only see artifacts, not implementation process
If implemented as specified, this would be our strongest differentiator. No tool in our landscape survey — including Agentic — provides comparable pipeline separation. Anthropic's research validates 90.2% improvement from multi-agent separation (context-engineering supplement).

6.2 Spec-Driven Methodology (ISO I→S→O)
Agentic's plans are flexible — any workflow can be expressed. But flexibility means methodology enforcement depends on plan authors making the right choices. A developer can write a plan that skips specification, testing, or review.

Our L2 state machine is designed to enforce the ISO I→S→O flow: every change would go through specify → plan → implement → review → verify. This is a governance constraint, not a convenience. If implemented, it would ensure:

Every change has a traceable specification
Every specification has a verification plan
Every verification produces an independent verdict
Agentic's dev-orchestration-v1 plan follows a similar flow by convention, but it's one plan among 36. Our architecture would make it the only allowed flow for ISO-governed work.

6.3 VE Independence Model
In Agentic, session groups can allow review agents to share conversation context with implementation agents. Even without session groups, review agents operate in the same workspace and may be influenced by implementation artifacts beyond the spec.

Our VE model is designed to enforce informational isolation: the verification agent would not have access to the implementation agent's reasoning, only its committed artifacts. This independence is critical for our quality model:

VE would write tests from the spec, not from the implementation
VE could not be biased by seeing the implementation approach
VE findings would carry weight precisely because they are independent
6.4 Governance-First Design
Agentic's governance is opt-in: add security.threatDetection: true to your plan YAML. Our design specifies mandatory governance: the Governance Engine would enforce 13 gates regardless of plan configuration. Key design differences:

Dimension	Agentic (implemented)	ISO Harness (designed)
Gate enforcement	Plan author decides	System-wide, mandatory
Release authority	No concept	QCOE would have independent release/hold authority
Hook enforcement	onFailure recovery handler only (advisory, post-retry)	Pre/post-execution hooks (would be blocking) — DA-09
Audit trail	Activity timeline per run	Full traceability: spec → verifier → verdict → gate → release
6.5 Tiered Execution Substrate
Agentic provides Docker or Podman — one tier. Our ADR specifies three tiers:

Tier	Substrate	Pool	Trust Level
1	runc (standard Docker)	Pool A (dev) — trusted agents, known repos	Standard
2	gVisor (kernel-level isolation)	Pool B (test) — semi-trusted, running arbitrary tests	Elevated
3	Kata (VM-level isolation)	Pool C (review) — untrusted, adversarial verification	Maximum
This tiered approach would balance performance (runc is fast) with security (Kata for adversarial workloads). Whether this granularity is needed in practice is a design assumption that production experience would validate.

6.6 Formal Requirements Traceability
Agentic tracks runs, steps, and costs. It does not track requirements. Our Traceability Engine is designed to maintain a formal mapping:

Requirement (spec.md) → Verifier (test/check) → Verdict (pass/fail) → Gate (approved/blocked)
This traceability chain, if implemented, would enable proving that every requirement was verified, every verification was independent, and every gate was satisfied.

7. Strategic Options
Update Note (post-ADR-004): This section was originally written under the premise that the ISO harness execution layer would be built from scratch. ADR-004 subsequently recognized DFT (Dark Factory Tool) as a production-tested execution engine — ~14K LOC, hundreds of specs processed, DAG scheduling, container lifecycle, retry heuristics, state management — that serves as the L3 foundation for the ISO harness. The analysis below has been recalibrated to reflect that the ISO path does NOT start from zero. Scoring and conclusions have changed accordingly.

Option A: Adopt Agentic as Orchestration Substrate
What: Use Agentic.Engine.Core as the orchestration engine. Layer ISO methodology as custom plans, profiles, and step types. Build missing containers (Traceability Engine, Governance Engine, Verdict Service) as extensions.

Pros:

Immediate access to production-grade orchestration, state management, crash recovery
RAG, eval, cost tracking, and observability come free
157+ specs prove the engine works at scale
Internal team for support and collaboration
Cons:

.NET dependency (our harness design is language-agnostic)
Agentic's plan model may not cleanly enforce L2 state machine semantics
Session groups conflict with DFT "fresh conversation" model
Governance is opt-in by design — retrofitting mandatory governance may fight the architecture
Extension points may not support our container isolation model
DFT already provides much of the orchestration value (DAG scheduling, container management, retry, state) — the marginal time savings over evolving DFT are smaller than originally estimated
Risk: Medium-High. Retrofitting mandatory governance and pipeline isolation into an opt-in architecture is architecturally difficult. The plan model's flexibility works against methodology enforcement. The incremental value over DFT is narrower than a ground-up comparison would suggest.

Option B: Evolve DFT into ISO Harness (Learn from Agentic, No Code Dependency)
What: Evolve DFT — our production-tested execution engine with DAG scheduling, container lifecycle, retry heuristics, and state management — into the full ISO harness. Study Agentic's patterns (error classification, session management, RAG, eval replay) and incorporate the best ideas. No Agentic code dependency.

Pros:

Clean architecture that enforces ISO methodology from the ground up
No .NET dependency
No architectural compromises for governance enforcement
Can adopt Agentic patterns without adopting Agentic constraints
DFT provides a proven execution foundation — not starting from scratch
DFT already handles: parallel container orchestration, language bootstrapping (Python/Go/JS/C/C++), retry/stall detection, spec-kit integration, workspace isolation
Hundreds of real-world specs processed — operational lessons already embedded in the codebase
Much shorter development timeline than originally assumed (engine exists; governance/L2 orchestration layer is the delta)
Cons:

Still requires L1/L2 orchestration layer development (state machine, governance gates, traceability)
Copilot coupling in DFT requires strangler-pattern migration (3 phases per ADR-004)
Python runtime adds to multi-runtime complexity alongside .NET (Agentic)
Risk: Low-Medium. DFT proves the execution model works. The remaining work is governance overlay and L1/L2 orchestration — complex but bounded. The strangler migration for Copilot decoupling is the primary technical risk (addressed in ADR-004 §3.3).

Option C: Two-Lane Model — Agentic as Unified L3 Engine (Lane 1 Standalone + Lane 2 Governed)
What: Establish a two-lane execution model with Agentic as the common L3 engine for both lanes. Lane 1 (Express): Agentic handles lightweight, non-ISO workflows (bug fixes, small enhancements, CI analysis, code review) with full autonomy — no governance overhead. Lane 2 (ISO Full): The ISO harness's L1/L2 orchestrator + governance layers wrap Agentic as a governed execution substrate for medium-to-large multi-spec projects requiring traceability, VE independence, and mandatory governance gates. DFT's operational knowledge (retry heuristics, language bootstrap, classification) transfers as configuration and container templates. See DD-11 (integration.md §11) for the full seam architecture.

Pros:

ISO harness gets clean governance-first architecture (L1/L2) with Agentic's production-proven execution (L3)
Teams can use Agentic today for express workflows — no waiting for ISO harness completion
Unified engine — same Agentic runtime powers both lanes; difference is governance above, not engine below
Agentic's strengths (observability, crash recovery, plan composition, diminishing-returns, UI) serve both lanes
Governance fires above L3, not inside it — no need to retrofit governance INTO Agentic
DFT's battle-tested operational knowledge (heuristics, bootstrap, timeouts) transfers as configuration
Multi-runtime simplification: removes Python from L3 execution path
Cons:

Governance seam complexity — must build L3 Adapter, ISO-constrained profile enforcement, evidence extraction
Agentic team dependency — may need collaboration for ISO-constrained mode support
DFT knowledge transfer requires active effort (config + extensions, not automatic)
Dual-state risk (Agentic SQLite vs. harness DB) requires clear authority model
Some DFT behavioral logic (stall detection, classification) requires custom step types, not just config
Risk: Low. Agentic is proven at execution; governance is L2's responsibility (already designed). The primary technical risk is the governance seam (L3 Adapter + ISO-constrained profile) — bounded and well-defined by DD-11. DFT remains as fallback if integration encounters blockers.

Recommendation
Option C (Two-Lane Model with Agentic as Unified L3) is confirmed as the strategic direction (formalized in ADR-004 and DD-11 in integration.md). The evolution from the original Option C (separate engines per lane) to the current form (unified Agentic L3 with governance wrapping) reflects the insight that governance is an L2 concern, not an L3 concern — L3 needs a clean execution substrate, not built-in governance.

Decision Matrix (Recalibrated with DD-11)
Criterion	Weight	Option A (Adopt Agentic as orchestrator)	Option B (Evolve DFT alone)	Option C (Two-Lane, Agentic L3)
Time-to-value	30%	★★★★★ (immediate)	★★★★ (DFT is production-ready; L2 overlay needed)	★★★★★ (Agentic for express now + Agentic as governed L3)
Governance enforcement	25%	★★ (retrofit risk)	★★★★★ (clean, built natively)	★★★★★ (governance fires at L2, above L3 — no retrofit needed)
Org maintenance cost	15%	★★★★ (one engine)	★★★★ (evolve existing, no new build)	★★★★★ (one execution engine for both lanes)
Retrofit complexity	15%	★★ (governance retrofit into opt-in model)	★★★ (Copilot decoupling via strangler)	★★★★ (L3 Adapter is new work, but bounded)
Adoption friction	15%	★★★ (.NET dependency, governance compromise)	★★★★ (familiar DFT, adds governance)	★★★★★ (same engine everywhere, right governance at right layer)
Weighted score	 	3.30	3.90	4.70
Key scoring changes from previous evaluation:

Option C org maintenance cost rose from ★★★ to ★★★★★ — No longer two separate execution engines (Python + .NET). One Agentic engine powers both lanes.
Option C retrofit complexity changed from N/A to ★★★★ — The L3 Adapter and ISO-constrained profile are new work, but the scope is bounded and well-defined (DD-11).
Option B becomes less attractive — If Agentic can serve as L3 with governance wrapping, DFT's 14K LOC of Python becomes technical debt rather than an asset.
Option A remains rejected — Direct adoption still requires retrofitting mandatory governance INTO Agentic's plan model. Option C avoids this by keeping governance ABOVE Agentic.
Why Option C Scores Highest (Revised)
Governance is an L2 concern, not an L3 concern. The key architectural insight: L3 doesn't need built-in governance — it needs a clean execution substrate that L2 can control. Agentic provides this. Governance enforcement (mandatory gates, VE sidecar, QCOE blocks) fires at L2, above the L3 boundary. This eliminates the "retrofit governance" problem entirely.
Unified engine eliminates operational divergence. Same Agentic runtime for both lanes. Teams learn one execution model. Observability, debugging, plan authoring — all consistent. The difference between lanes is governance ABOVE, not engine BELOW.
Agentic provides immediate value at both levels. Lane 1: full-autonomy express workflows today. Lane 2: production-proven execution (crash recovery, DAG scheduling, diminishing-returns, observability) immediately available as L3 substrate. No waiting for DFT evolution.
DFT knowledge is preserved, not code. DFT's battle-tested operational heuristics (retry caps, language bootstrap, classification taxonomies) transfer as configuration to Agentic plans and L3 Adapter policies. The knowledge survives; the runtime dependency doesn't.
Specific Agentic capabilities now serve both lanes directly:
Error classification (10 categories) → L3 error recovery
Diminishing-returns detection → L3 iteration budget management
Plan composition (fragments) → L3 intra-stage task templates
Crash recovery (checkpoints) → L3 intra-stage resilience
Cost tracking per step → Observability Plane integration
Multi-surface UI → Developer experience for both lanes
Feasibility Spike (No Longer Relevant)
The original evaluation proposed a time-boxed spike to test whether Agentic could enforce mandatory governance. With DD-11's architectural insight (governance fires at L2, not inside L3), this spike is obsolete — we no longer need Agentic to enforce governance internally. The relevant Session 5 investigations are:

Can Agentic's API support programmatic invocation by the L3 Adapter (bypassing user-trigger surfaces)?
Can an ISO-constrained Agentic profile reliably restrict capabilities (no self-merge, no session reuse, no cross-run memory)?
How should the L3 Adapter extract evidence from Agentic's output into harness evidence format?
8. Impact on Design Actions
DA Reassessment in Light of Agentic
DA	Recommendation	Agentic Impact	Revised Status
DA-01	Three-layer instruction model	Agentic has no equivalent. Still needed.	No change
DA-02	Spec archival on merge	Agentic doesn't manage post-merge spec lifecycle. Still needed.	No change
DA-03	Agent→human request model	Agentic's checkpoint gates handle planned approvals with timeout policies (auto-approve/reject/cancel). But checkpoints are plan-authored pause points, not a structured request_human_input model that agents can invoke during unplanned escalations. Learn from their timeout policies; our model remains needed for unplanned escalation.	Partially related
DA-04	Spec-Kit extension evaluation	Agentic uses .specify/ — confirms Spec-Kit is the right foundation. Validate compatibility.	Enhanced
DA-05	Structured context format	Agentic uses plain text truncation, not structured assembly. Still needed.	No change
DA-06	Token budgeting per stage	Agentic has model-aware byte budgets but no per-component caps. Still needed, but adopt Agentic's model-specific budget table as a starting point.	Enhanced
DA-07	Proactive backpressure	Agentic doesn't compress tool outputs. Still needed.	No change
DA-08	Inner-sandbox pattern	Agentic uses host/none network modes (binary). Still needed — our proxy-based allowlist filtering is more granular.	No change
DA-09	CONSTITUTION.md hooks	Agentic has onFailure recovery handlers (advisory, post-retry only — see §10 correction). Still needed — our blocking enforcement model addresses a different concern (mandatory pre/post-stage gates, not error recovery).	No change
DA-10	Input content analysis	Agentic's threat detection is artifact/app security scanning (CodeQL/Semgrep/ZAP on generated code), not input content analysis (prompt injection defense on ingested content). Different control surface. Still needed. Separately, adopt Agentic's multi-scanner approach for post-generation security scanning as an additional governance layer.	No change (+ new note)
DA-11	SWE-ReX for substrate	Agentic's SandboxProcessService is a simpler alternative. Evaluate both — SWE-ReX for substrate abstraction, Agentic's pattern for Docker/Podman simplicity.	Broadened
DA-12	Tiered routing cascade	Agentic doesn't do this. Still needed.	No change
DA-13	Per-call cost ledger	Partially resolved. Agentic's per-step token tracking with per-model breakdown proves the pattern works. But our DA-13 also includes infrastructure cost per container-hour, cascade-tier resolution tracking, and multi-dimensional aggregation (per-spec, per-pool, per-developer). Adopt Agentic's token tracking; extend for infrastructure and aggregation.	Enhanced
DA-14	Trace-based grader taxonomy	Agentic has eval replay but no multi-tier grading. Still needed, but adopt their replay mechanism as the execution substrate for our graders.	Enhanced
DA-15	Infrastructure noise margins	Agentic doesn't address this. Still needed.	No change
DA-16	Eval lifecycle management	Agentic doesn't address this. Still needed.	No change
DA-17	Harness self-evolution	Agentic doesn't address this. Still needed.	No change
Summary: Of 17 design actions:

5 enhanced (DA-04, DA-06, DA-13, DA-14 — learn from Agentic's implementations; DA-03 — partially related, learn timeout policies)
1 broadened (DA-11 — evaluate Agentic's approach alongside SWE-ReX)
11 unchanged — design gaps that Agentic doesn't address (DA-01, DA-02, DA-05, DA-07, DA-08, DA-09, DA-10, DA-12, DA-15, DA-16, DA-17)
New Considerations from Agentic
ID	Consideration	Source	Priority
AC-01	RAG for Context Assembler — Agentic's RAG subsystem (vector store, multi-embedding, language-aware chunking) would significantly improve our Context Assembler's ability to select relevant code.	Agentic RAG subsystem	HIGH
AC-02a	Declarative workflow authoring — L2 pipeline stage definitions SHOULD be expressed as composable, version-controlled YAML (fragments with typed parameters). Agentic's fragment composition model (§2 Plan Composability) proves this pattern is production-mature. The YAML definitions serve as the configuration surface; the governance-aware L2 state machine remains the enforcement engine.	Agentic fragment model, specs/110-plan-composition	HIGH
AC-02b	YAML as runtime execution source-of-truth — Whether the L2 state machine should interpret YAML directly at runtime (vs. compiling YAML to internal representation at build time). Agentic's parse-time resolution model (fragments inlined before execution) demonstrates that static composition is sufficient for fixed-sequence pipelines.	Agentic PlanParser	MEDIUM
AC-03	Diminishing-returns detection — Agentic's convergence evaluation across cycles is more sophisticated than our consecutive-failure stall detection. Adopt for our L3 iteration budget.	Agentic CycleRunner	MEDIUM
AC-04	Agent error classification — Agentic's 10-category error taxonomy with remediation steps should be adopted for our Agent Gateway error handling design.	SdkErrorClassifier	MEDIUM
AC-05	Cross-run memory — Agentic's MemoryService with confidence scoring could improve our agents' ability to learn from past spec executions.	Agentic MemoryService	LOW
AC-06	Fragment composition as ISO reference architecture — Agentic's fragment model (typed parameters, multi-import aliasing, per-step overrides, circular dependency detection) should serve as the reference architecture for our L2/L3 workflow templating system. When designing the ISO harness's plan authoring layer, adopt this composition model for stage templates and reusable workflow building blocks.	Agentic fragment model, specs/110-plan-composition	HIGH
10. Source-Code Verification Register
This section documents which evaluation claims have been verified against Agentic source code (April 2026), and where the original assessment required correction. Transparency about verification status prevents decisions based on assumed capabilities.

Claim	Original §	Verification	Status	Correction
Plan-as-Code model	§2	specs/110-plan-composition/, fragment YAML files	✅ Verified & Expanded	Fragment composition is more mature than originally described — typed params, multi-import, aliasing, parse-time resolution
onStepStart/onStepEnd/onCycleEnd lifecycle hooks	§4.6	specs/064-step-onfailure-handler/spec.md	❌ Corrected	Only onFailure exists. Advisory recovery only — cannot abort/skip/retry. No lifecycle hooks.
IAgentProvider interface	§2	src/Agentic.Shared/Contracts/IAgentProvider.cs	✅ Verified	Clean agent-backend abstraction. Per-step provider override confirmed.
DAG scheduling (Kahn's algorithm)	§4.1	src/Agentic.Engine.Core/Services/StepDependencyGraph.cs	✅ Verified	Parallel waves, failure propagation, continueOnFailure semantics confirmed
4 execution surfaces	§5	GitHub Actions composite, CLI, Web (Blazor), MAUI projects	✅ Verified	CLI + Web + Desktop + GitHub Actions. All share same engine.
Checkpoint crash recovery	§3, §5	SQLite via EF Core, checkpoint persistence	✅ Verified	Survives crashes, resumes from exact step
Cross-repo coordination	§4.1	propagate-changes plan	✅ Verified	Confirmed as shell-script workaround, not first-class feature
Governance opt-in	§4.6	Plan YAML security.threatDetection field	✅ Verified	Per-plan opt-in confirmed. No system-wide mandatory enforcement.
Provider vs. substrate pluggability	§2, §5	IAgentProvider vs. SandboxProcessService	✅ Clarified	Originally conflated. Now separated into three pluggability dimensions (§2 Execution Pluggability Clarification).
Methodology: Claims were verified by reading source code, spec documents, and test files in the ncrvoyix-swt-cfr/agentic repository. Where source evidence contradicted the original evaluation, corrections were made inline with (source-verified) annotations. Where source evidence confirmed claims, the original text was preserved unchanged.

11. Cross-References
Document	Section	Relationship
integration.md	§3 Orchestrator Engine	Direct comparison with Agentic's orchestration model
integration.md	§8 Context Assembly	Compared to Agentic's context windowing
integration.md	DD-10	Declarative Workflow Definition — informed by Agentic fragment model
integration.md	DD-11	L3 Engine Selection — Agentic as governed execution substrate
architecture.md	Containers & components	§3 capability mapping reference
architecture.md	§7 VE Isolation	Key differentiator absent from Agentic
landscape-design-actions.md	All 17 DAs	§8 DA reassessment
execution-substrate.md	Tiered substrate	Compared to Agentic's Docker/Podman single-tier; §2 clarifies provider vs. substrate distinction
workspace-lifecycle.md	DFT model	Tension with Agentic's session groups
dft-heritage-two-lane.md	ADR-004: DFT Heritage & Two-Lane Model	Formalizes Option C; Agentic as unified L3 engine (DD-11); DFT as knowledge source; §3.4-3.5 layer boundary contracts
runtimes-implementations.md	External tool evaluations	Agentic evaluation complements external landscape
specs-workflow-design.md	12-Factor audit	Many factors addressed by Agentic's implementation
overview.md	§6 Industry Patterns	Agentic implements several patterns (plan-as-code, durable execution, multi-agent)
End of evaluation. Key takeaway: Agentic is the unified L3 execution engine for both lanes — standalone in Lane 1, governed by L2 in Lane 2. The architectural insight: governance is an L2 concern, not an L3 concern. Agentic's execution maturity (observability, crash recovery, plan composition, diminishing-returns detection, multi-surface UI) serves both lanes directly. The L2 state machine, governance gates, VE sidecar, and traceability layers fire ABOVE Agentic via the L3 Adapter (DD-11). DFT's operational knowledge (heuristics, bootstrap, classification) transfers as configuration — not code dependency. The settled architecture is a governance seam model: same engine below, different governance above.

