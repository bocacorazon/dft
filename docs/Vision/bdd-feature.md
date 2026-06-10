Feature Specification: BDD Verification with godog
| Feature ID | bdd-verification |
| Version | 1.0.0 |
| Status | Draft |

1. Summary
Integrate godog (Cucumber BDD for Go) as the verification execution engine
for the ISO toolkit. Express verification conditions as standard Gherkin
.feature files. Wire existing adapter hooks (CLI, API, Web UI — 16 hooks
total) as godog step definitions. Build a plan-driven orchestrator that reads
verification plans, executes godog suites, and produces structured
feature-scope verdicts with requirement coverage mapping.

This increment completes the verification pipeline by connecting all existing
building blocks — scenario runner, adapters, verdict aggregator, verification
plan schema, and remediation classifier — into an executable end-to-end
verification workflow.

2. Background
The verification layer was built in 10 specs (S00–S09) across four tracks:
contracts and planning, scenario execution core, verdict and remediation,
and expansion and taxonomy. Each component works in isolation — adapters
execute steps, the runner dispatches to adapters, the aggregator produces
verdicts, the plan schema models requirements. However, no integration glue
exists to orchestrate these components into a verification pipeline.

The current verdict run command generates AI-authored Go tests from spec
acceptance criteria. While functional, this approach lacks: a human-readable
test format, requirement-level coverage tracking, and integration with the
verification plan schema. BDD/Gherkin provides a standard, legible format
for expressing verification conditions that maps naturally to the existing
Given/When/Then phase model already implemented in internal/scenario/.

godog is the standard Cucumber implementation for Go. It is battle-tested,
runs within go test, and supports structured output formats (cucumber JSON)
suitable for programmatic consumption.

3. Requirements
User Story 1 - godog Step Definition Bridge (Priority: P1)
As a verification engineer, I need all 16 existing adapter hooks available
as godog step definitions so that I can write Gherkin .feature files that
exercise CLI commands, HTTP APIs, and Web UI interactions using the standard
BDD Given/When/Then format.

The internal/bdd/ package provides InitializeScenario(cfg SuiteConfig)
which returns a function that registers all step definitions with a godog
ScenarioContext. SuiteConfig accepts an *http.Client for the API
adapter, a webui.Driver for the Web UI adapter, and a BaseDir string
for the CLI adapter's working directory.

Each step definition:

Constructs a scenario.Step with the correct Adapter, Hook, Phase,
and Params fields
Calls the appropriate adapter's Execute(ctx, step) method
Returns nil on pass or a descriptive error on failure (as godog expects)
Stores per-scenario mutable state (last HTTP response, last exit code,
last stdout, working directory) in a suite struct scoped to the scenario
Step patterns registered (16 total):

CLI adapter (6 hooks):

Given an environment variable "{key}" set to "{value}" → cli.env
Given a file "{path}" with content: (docstring) → cli.write_file
When I run "{command}" → cli.exec
Then the exit code should be {code:d} → cli.expect_exit_code
Then the output should contain "{text}" → cli.expect_stdout
Then the file "{path}" should exist → cli.expect_file
API adapter (5 hooks):

Given a seeded "{type}" at "{endpoint}" → api.seed
When I send a "{method}" request to "{url}" → api.request
Then the response status should be {code:d} → api.expect_status
Then the response JSON at "{path}" should be "{value}" → api.expect_json
Then a side effect at "{url}" should match "{pattern}" → api.expect_side_effect
WebUI adapter (5 hooks):

Given I navigate to "{route}" → webui.route
When I click "{selector}" → webui.click
When I type "{text}" into "{selector}" → webui.type
Then I should see "{text}" → webui.expect_text
Then the element "{selector}" should be visible → webui.expect_visible
Acceptance Scenarios:

Given a godog test suite with SuiteConfig containing a FakeDriver and a temp directory, When a .feature file exercises all 16 step patterns, Then every step definition dispatches to the correct adapter hook and all steps pass
Given a godog test suite with SuiteConfig where Driver is nil, When a .feature file contains a WebUI step (I navigate to), Then the step returns a descriptive error indicating WebUI adapter is unavailable
Given two scenarios running sequentially in the same godog suite, When the first scenario sets an environment variable and the second does not, Then the second scenario does not inherit the first scenario's state (per-scenario isolation)
Given a .feature file with a CLI step When I run "false", When the step executes, Then the exit code is captured and Then the exit code should be 0 fails with a message containing the actual exit code
User Story 2 - Plan-Driven BDD Orchestrator (Priority: P1)
As a verification engineer, I need an orchestrator that reads a verification
plan YAML, locates feature file directories, runs godog suites per pack, and
aggregates results into a feature-scope verdict so that I can verify an
entire feature increment with a single command.

The internal/verdictrun/ package provides:

type OrchestratorConfig struct {
    PlanPath   string
    DemandPath string          // optional — enables requirement coverage
    BaseDir    string          // root for resolving pack Source paths
    HTTPClient *http.Client
    Driver     webui.Driver    // nil → skip WebUI-only packs
}

type OrchestratorResult struct {
    Verdict    *verdict.FeatureVerdict
    Executions []PackExecution
    Coverage   *RequirementCoverage   // nil when DemandPath is empty
}

type PackExecution struct {
    PackID   string
    Source   string
    Status   string          // "pass", "fail", "error", "skipped"
    Duration time.Duration
    Outcomes []verdict.ScenarioRunnerOutcome
}

type RequirementCoverage struct {
    Total     int
    Covered   int
    Uncovered []string
}
Orchestrate(ctx, cfg) performs:

Load verification plan via verifyplan.LoadVerificationPlan(cfg.PlanPath)
Optionally load demand package from cfg.DemandPath for coverage
For each PackManifest in the plan:
a. Resolve Source relative to cfg.BaseDir to a feature directory
b. Create a godog TestSuite with bdd.InitializeScenario(suiteCfg)
c. Configure godog options: Paths, Format: "cucumber", NoColors: true
d. Run the suite — capture exit status and structured output
e. Parse cucumber JSON output → bridge to verdict.ScenarioRunnerOutcome
f. Record PackExecution with status, duration, outcomes
Collect all outcomes, call verdict.AggregateFeatureVerdict()
If demand package loaded, compute RequirementCoverage:
Match pack scenario tags (@REQ-XXX) to demand requirement IDs
Covered = requirement IDs with at least one passing scenario
Uncovered = requirement IDs with no matching scenario tags
Acceptance Scenarios:

Given a verification plan with two scenario packs and corresponding .feature directories, When Orchestrate is called, Then the result contains a FeatureVerdict with status PASS and two PackExecution entries with status "pass"
Given a verification plan referencing a pack Source that does not exist on disk, When Orchestrate is called, Then the PackExecution for that pack has status "error" and the FeatureVerdict status is FAIL
Given a verification plan and a demand package with 3 requirements, When 2 requirements have matching scenario tags and 1 does not, Then RequirementCoverage shows Total=3, Covered=2, and Uncovered contains the unmatched requirement ID
Given a verification plan with a pack whose scenarios all use WebUI steps and Driver is nil in config, When Orchestrate is called, Then that pack's PackExecution has status "error" with a message about missing WebUI driver
User Story 3 - Auto Hook Selection Engine (Priority: P2)
As a demand package author, I need the system to automatically suggest which
hook family to use for a requirement based on its stable surface tokens so
that I do not have to manually look up adapter capabilities for common cases.

The internal/hookselect/ package provides:

type SelectionResult struct {
    Family     verifyplan.HookFamily
    Confidence float64    // 0.0–1.0
    Reasoning  string
    Escalate   bool       // true when confidence < threshold
}

func Select(surfaces []string) SelectionResult
func SelectWithThreshold(surfaces []string, threshold float64) SelectionResult
Pattern rules (evaluated in order, first strong match wins):

Surfaces containing HTTP methods (GET, POST, PUT, DELETE, PATCH)
or URL-like patterns (/api/, http://, :port/) → api family
Surfaces containing command prefixes (cmd:, iso, go, git,
verdict) or shell patterns → cli family
Surfaces containing UI selectors (data-testid:, route:/, #, .class,
[role=) → web-ui family
Surfaces containing messaging tokens (topic:, queue:, exchange:) → event
Surfaces containing file tokens (path:, glob:, bucket:) → file
No match or ambiguous signals → Escalate: true, Confidence: 0.0
When multiple families match, confidence is reduced proportionally and
the dominant family (most signal matches) is returned. Threshold default
is 0.6.

Acceptance Scenarios:

Given surfaces ["cmd:iso validate", "flag:--plan"], When Select is called, Then Family is cli, Confidence >= 0.8, and Escalate is false
Given surfaces ["POST /api/v1/jobs", "Content-Type: application/json"], When Select is called, Then Family is api, Confidence >= 0.8, and Escalate is false
Given surfaces ["data-testid:submit-btn", "route:/jobs/new"], When Select is called, Then Family is web-ui, Confidence >= 0.8, and Escalate is false
Given surfaces ["some-unknown-surface"], When Select is called, Then Escalate is true and Confidence is 0.0
Given surfaces ["cmd:curlhttp://localhost/api", "route:/"], When Select is called with mixed CLI and API signals, Then Confidence is below 0.8 reflecting ambiguity
User Story 4 - Hook Scaffolder (Priority: P3)
As a developer extending the verification layer, I need a code generator
that produces skeleton Go code for a new adapter hook — including the handler
function, godog step definition, test stub, and KnowsHook entry — so that
I can add hooks following established patterns without manual boilerplate.

The internal/hookscaffold/ package provides:

type ScaffoldRequest struct {
    Family    verifyplan.HookFamily
    HookName  string
    Phase     scenario.Phase
    StepText  string    // Gherkin pattern, e.g., `the header "{name}" should be "{value}"`
}

type ScaffoldResult struct {
    HandlerCode string
    StepDefCode string
    TestCode    string
    KnowsHook  string
    TargetDir   string
}

func Scaffold(req ScaffoldRequest) (*ScaffoldResult, error)
Generated code follows the conventions of existing adapters:

Handler: func execHookName(ctx *ScenarioCtx, step scenario.Step) scenario.StepResult
StepDef: sc.Step(\^pattern$`, s.hookName)`
Test: table-driven test with pass/fail cases
KnowsHook: case "hook_name": added to switch
Acceptance Scenarios:

Given a ScaffoldRequest for family api, hook expect_header, phase then, When Scaffold is called, Then HandlerCode contains a function signature matching existing api adapter patterns and TargetDir is internal/verifyhooks/api/
Given a ScaffoldRequest for family cli, hook expect_stderr, phase then, When Scaffold is called, Then all four code sections (handler, step def, test, KnowsHook) are non-empty and syntactically valid Go
Given a ScaffoldRequest with an unknown family nonexistent, When Scaffold is called, Then it returns a descriptive error
User Story 5 - Feature File Generator Agent (Priority: P2)
As a verification engineer, I need an agent that generates .feature files
from demand requirements so that I can produce BDD verification scenarios
without manually authoring Gherkin for every requirement.

The agent definition at .github/agents/iso.feature-generator.agent.md
reads demand-requirements.yaml and produces Gherkin .feature files at
testdata/features/<slug>/. Each requirement's acceptance criteria, hook
family, and stable surface are transformed into Gherkin scenarios.

The companion template at docs/templates/feature-file-template.md provides:

Complete reference of all 16 registered step patterns with Gherkin syntax
Mapping table: adapter name ↔ hook family ↔ available hooks ↔ step text
Tagging conventions: @REQ-XXX for requirement traceability, @wip for
incomplete scenarios, @smoke for quick validation
Example .feature files showing CLI, API, and WebUI scenarios
Acceptance Scenarios:

Given the agent definition exists at .github/agents/iso.feature-generator.agent.md, When parsed as markdown, Then it contains valid frontmatter with a description field
Given the template at docs/templates/feature-file-template.md, When read, Then it documents all 16 step patterns with exact Gherkin syntax matching internal/bdd/ registrations
Given a demand-requirements.yaml with 3 CLI-family requirements, When the agent generates feature files, Then each requirement produces at least one @REQ-XXX tagged scenario using CLI step patterns
User Story 6 - CLI Surface: verdict run --plan (Priority: P1)
As a verification engineer, I need a --plan flag on the verdict run
command that switches from AI test generation to plan-driven BDD execution
so that I can run structured verification plans from the command line.

The existing cmd/verdict/run.go gains:

--plan string flag: path to verification-plan.yaml
--demand string flag: optional path to demand-requirements.yaml
When --plan is provided:

Skip all AI test generation logic (no gateway call, no temp dir)
Call verdictrun.Orchestrate(ctx, cfg) with the plan and optional demand
Marshal OrchestratorResult.Verdict to JSON
Write to --output file or stdout
Set exit code based on verdict status: 0=PASS, 1=FAIL, 2=ERROR
When --plan is NOT provided, the existing AI test generation workflow
executes unchanged (full backward compatibility).

Acceptance Scenarios:

Given a valid verification plan and feature files, When verdict run --plan plan.yaml is executed, Then structured FeatureVerdict JSON is written to stdout and exit code is 0 for a passing plan
Given a verification plan where one pack fails, When verdict run --plan plan.yaml is executed, Then exit code is 1 and the JSON verdict contains status "FAIL" with evidence
Given verdict run is called WITHOUT --plan, When executed with --spec and --artifact flags, Then the existing AI test generation workflow runs unchanged
Given a verification plan and a demand package, When verdict run --plan plan.yaml --demand demand.yaml --output result.json is executed, Then the output file contains a FeatureVerdict JSON with a RequirementCoverage section
4. Non-Functional Requirements
Performance: godog suite execution for a single pack with 10 scenarios
must complete in under 30 seconds (excluding actual command/HTTP execution time)
Concurrency safety: The BDD suite struct is per-scenario (not shared).
Adapter calls are safe for concurrent use (existing contract).
Observability: Pack execution logs pack ID, scenario count, and duration
to stderr. Failed steps include adapter name, hook, and failure message.
Compatibility: godog v0.14+ is the target version. The godog dependency
must not introduce transitive dependencies that conflict with existing go.mod.
5. Edge Cases and Error Handling
Missing feature directory: When a pack Source resolves to a nonexistent
path, the orchestrator records status "error" for that pack and continues
to the next pack. The overall verdict is FAIL.
Empty feature directory: When a resolved directory contains no .feature
files, godog reports no scenarios found. The orchestrator records status
"skipped" and treats it as non-failing (vacuous truth).
WebUI driver nil: When SuiteConfig.Driver is nil and a scenario contains
WebUI steps, each WebUI step definition returns an error indicating the
driver is unavailable. The scenario fails cleanly.
Malformed feature file: godog returns a parse error. The orchestrator
captures it and records status "error" for the pack.
Context cancellation: If the parent context is cancelled during
Orchestrate, the current pack execution is abandoned and the function
returns the partial result with an error.
No packs in plan: An empty plan (no PackManifests) produces a vacuously
passing FeatureVerdict with empty Evidence.
Duplicate requirement tags: Multiple scenarios tagged with the same
@REQ-XXX are valid — coverage counts the requirement as covered if at
least one tagged scenario passes.
6. Configuration
godog version: github.com/cucumber/godog v0.14+ (latest stable)
Feature file location: testdata/features/<feature-slug>/ relative
to repository root
Default confidence threshold: 0.6 for auto hook selection
Cucumber JSON format: godog Format: "cucumber" for structured output
Exit codes: verdict run --plan uses 0/1/2 matching existing convention
7. Dependencies
Internal (consumed, not modified):
internal/scenario — Adapter, Step, Phase types
internal/verifyhooks/api — API adapter and context
internal/verifyhooks/cli — CLI adapter and context
internal/verifyhooks/webui — WebUI adapter, Driver, FakeDriver
internal/verifyplan — VerificationPlan, DemandPackage, HookFamily, loader
internal/verdict — FeatureVerdict, AggregateFeatureVerdict, OutcomeFromRunResult
cmd/verdict — existing cobra root command and run subcommand
External (new):
github.com/cucumber/godog — BDD test runner (only new dependency)
