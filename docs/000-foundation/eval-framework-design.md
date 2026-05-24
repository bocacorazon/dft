# Artifact-Only BDD Eval Framework

## Purpose

The eval framework verifies a completed demand package after all WBS entries have finished and merged into the increment branch. Eval is adversarial and artifact-only: the eval agent reads the demand, acceptance criteria, WBS/spec artifacts, and declared evaluation surfaces, but it does not read implementation source code or implementation-agent transcripts.

The eval agent decides what behavior to verify. The engine and project contracts decide where the delivered artifact is available and when it is ready for testing.

## Current baseline

The current implementation has an early deterministic evaluator:

- `internal/review/EvalPlanAuthor` asks `dft-eval-plan-author.agent.md` for a flat `domain.EvaluationPlan`.
- `internal/review/Evaluator` executes that plan through `ports.Verifier`.
- `internal/domain/verification.go` models `EvaluationPlan{Checks []Check}` and `VerificationResult`.
- `internal/adapters/verify/checker.go` supports local checks such as file existence, grep, JSON path, command exit, git state, and binary-artifact detection.

That remains useful as a deterministic check family, but it is too narrow for demand-package eval. The new design needs artifact/surface contracts, BDD packs, readiness gates, requirement coverage, and hidden eval handling.

## Implemented foundation

The first framework slice is implemented in:

- `internal/domain/eval.go`: surface contracts, artifact manifests, readiness results, BDD packs, eval plans, eval results, coverage, and evidence references
- `internal/eval/surface_author.go`: design-time eval surface authoring to `.dft/runs/<run-id>/design/eval-surfaces.json`
- `internal/eval/readiness.go`: surface-to-artifact binding plus readiness probes
- `internal/eval/author.go`: source-blind artifact-only eval-plan authoring
- `internal/eval/executor.go`: CLI, file, HTTP, and deterministic-check execution
- `internal/eval/orchestrator.go`: reusable readiness -> plan author -> executor phase component
- `internal/eval/legacy.go`: adapters between the old deterministic `EvaluationPlan`/`VerificationResult` and the new eval model
- `internal/orchestration/macro.go`: macro-loop eval gate now runs the new artifact-only eval orchestrator before final review/merge

## Core design

The missing abstraction is an explicit **evaluation surface contract**. The eval agent must not infer where to test by inspecting code. Instead, solution design produces a contract describing observable surfaces and their target bindings.

The default target is **ephemeral artifact/container evaluation**. Bound external and live systems are opt-in only through explicit surface contracts.

## Evaluation surface contract

Surface contracts are authored during solution design alongside the WBS. Every planned spec should be tied to the artifact or observable surface that will prove it.

A surface entry should include:

- `surface_id`
- `kind`: `cli`, `http_api`, `graphql`, `grpc`, `web_ui`, `file`, `event`, `database`, `infra`, `container`, or `composite`
- `artifact_ref`: binary path, container image, base URL, schema file, queue/topic name, output file/glob, or similar
- `adapter_family`: BDD execution adapter
- `environment_class`: `ephemeral`, `bound_external`, or `live`
- `provisioning`: build artifact, container image, docker compose, existing URL, pre-created queue, or none
- `readiness`: command exit zero, HTTP health check, file exists, TCP open, queue reachable, container health status, or other closed-set probe
- `reset_policy`: seed and cleanup behavior
- `evidence_policy`: artifacts/logs captured after execution

If a required surface is missing or ambiguous, eval blocks with a structured finding instead of guessing.

## Artifact manifest and readiness

After all WBS specs complete and merge into the increment branch, the engine collects or creates an artifact manifest:

- built CLI binaries
- container images
- generated files
- public API schemas
- deployment URLs for external/live targets
- queue/topic bindings
- database/schema artifacts

Eval starts only after an `eval-ready` artifact proves:

- all WBS entries are complete
- all spec branches merged back to the increment branch
- the increment worktree is clean enough for evaluation
- the artifact manifest exists
- every required surface has a bound artifact or endpoint
- all readiness probes pass

Readiness is an engine gate, not an eval-agent judgment.

## Artifact-only eval-plan author

The eval-plan author receives:

- demand package title, raw demand, assumptions, and non-goals
- acceptance criteria
- WBS/spec descriptions and acceptance criteria
- allowed process artifacts such as generated spec summaries
- evaluation surface contract
- artifact manifest and readiness metadata
- allowed BDD step catalog / adapter families

The eval-plan author does not receive:

- implementation source files
- implementation diffs
- implementation-agent transcripts
- full code-review transcripts

It emits:

- BDD feature packs using Gherkin-style scenarios
- requirement tags such as `@REQ-...`
- a structured eval plan mapping packs to surfaces/adapters/artifacts
- unsupported or ambiguous criteria as findings

Generated BDD packs are hidden from implementation agents by default.

## BDD execution model

dft owns a generic BDD/eval plan model. `godog` can be the first executor for Gherkin packs, but it stays behind an adapter so the domain model is not coupled to godog concepts.

The first execution slice should focus on:

- `cli`: run delivered binary/argv, assert exit code/stdout/stderr/files
- `file`: assert generated artifacts, contents, JSON/YAML paths, checksums
- `http_api`: call a bound base URL, assert status/body/headers/JSON paths
- `container`: start/inspect image or container health as a provisioning helper

Later adapter families can add Web UI, event queues, databases, infra policy, GraphQL, gRPC, and composite systems.

## Verdict and traceability

The new eval result should be requirement-traceable:

- status: `pass`, `fail`, `error`, or `blocked`
- pack executions
- scenario outcomes
- requirement coverage
- findings
- evidence references

Every finding should identify:

- requirement or acceptance criterion
- tested surface
- scenario/check ID
- evidence artifact
- severity/category/advice where applicable

Hidden eval failures disclose only requirement, surface, assertion summary, and evidence references to Fix-Planner. Full hidden scenario text and adversarial inputs remain hidden unless a human explicitly opens the eval artifact.

## Macro-loop integration

The eval phase becomes:

1. Confirm WBS/spec orchestration is complete.
2. Build/package/collect delivered artifacts.
3. Bind evaluation surfaces to artifacts/environments.
4. Run readiness probes and write `eval-ready.json`.
5. Invoke artifact-only eval-plan author.
6. Execute BDD packs deterministically.
7. Aggregate verdict and requirement coverage.
8. If fail/block/error, invoke Fix-Planner with structured findings.
9. If pass, allow final review/merge.

Current implementation note: the first artifact manifest is derived from the design-time surface contract. This supports local artifact/file/URL bindings immediately and gives later build/package work a stable manifest seam to replace or enrich.

## Initial implementation workstreams

1. Define eval domain models for surfaces, artifact manifests, readiness probes, BDD packs, eval plans, eval results, findings, and coverage.
2. Define the artifact-only eval-plan author contract and enforce source exclusion by construction.
3. Implement surface binding and readiness against fake/local targets first.
4. Implement the first BDD executor slice for CLI, file, and HTTP surfaces.
5. Implement verdict aggregation and requirement traceability.
6. Migrate the existing deterministic evaluator into the new eval framework as one check/adapter family.
