# DFT Front-Agent Environment Design Plan

> **For Hermes:** Use `plan` skill only. This is a design document, not an implementation plan.

**Goal:** Design where and how DFT front agents live, how they plug into Hermes, and how they produce contract-compliant artifacts for DFT's execution layer.

**Architecture:** Front agents become Hermes skills living inside the DFT repo under `hermes/skills/`, each with embedded JSON-schema references matching DFT v2 domain types. A master `dft-design` skill orchestrates Intent → Solution phases and outputs `demand-package.json` and `solution-design.json` validated against DFT contracts. The DFT harness changes from embedding a monolithic prompt to invoking `hermes -w chat --skill dft-design`.

**Tech Stack:** Hermes skill system (SKILL.md + YAML frontmatter), DFT v2 domain types (Go structs → JSON schemas), bash/curl for import tooling.

---

## Context & Assumptions

### What DFT does today
- **Design phase (Intent + Solution)**: The eval harness (`internal/eval/harness/hermes_agent.go`) shells out to `hermes -w chat` with a monolithic 72-line prompt that tells Hermes to produce `demand-package.json` and `solution-design.json` in one shot.
- **Build phase**: DFT's flow runner invokes agents by name (e.g., `speckit.specify.agent.md`) via the `AgentAdapter` port. These are GitHub Copilot-specific `.agent.md` files living at `internal/app/provisioned/github/agents/`.
- **Contracts**: Defined in `internal/domain/v2/` as Go structs: `DemandPackage`, `SolutionDesign`, `TestPlan`, `WBS`, `EvalSurfaceContract`, `OrchestrationResult`. Each has a `.Validate()` method.

### What the user wants
- DFT becomes execution-only. No design logic lives in DFT's Go code.
- Front agents (Intent, Solution phases) live in a Hermes-native environment as skills.
- Agents know how to format their outputs to meet DFT's JSON contracts.
- The overall workflow is skill-driven, not opaque prompt-driven.

### Key architectural constraint
DFT currently has two distinct agent invocation paths:
1. **Design path**: `harness/hermes_agent.go` → `hermes -w chat -q "<monolithic prompt>"` (one-shot, untyped)
2. **Execution path**: `flow.Runner` → `AgentAdapter.Invoke(AgentName="speckit.specify.agent.md")` (typed, per-stage)

The front agents concern path 1 only. Path 2 agents (speckit.*.agent.md) remain as-is for execution — the user said those are outside scope.

---

## Design Decisions

### Decision 1: Where do front-agent skills live?

**Option A**: Inside the DFT repo at `hermes/skills/dft-*/SKILL.md`
**Option B**: In `~/.hermes/skills/dft-*/SKILL.md` (outside the repo)
**Option C**: A separate `dft-agents` repo

**Recommendation: standalone central repo**, not embedded in DFT.

The design agents live in `dft-design-agents/` — a single repo shared across all projects. One symlink per machine (`ln -s ~/dft-design-agents/skills/dft-* ~/.hermes/skills/`). Projects consume them as Hermes skills with zero per-project setup.

Rationale for central over per-project:
- Skills are genuinely generic. They read project context at runtime (constitution, surfaces, codebase) but encode no project-specific behavior. The same skill works for a Go CLI tool, a Python web service, or a Terraform module.
- Bug fixes propagate instantly: update the central skill, every project gets the fix on next invocation. No per-project `git pull` / copy dance.
- Contract compliance is enforced by DFT's `Validate()` at runtime, not by schema files. The design agents don't carry DFT version baggage — they call `dft schema <type>` at runtime to discover the current contract.
- The coupling surface is minimal: skills produce JSON that passes DFT's `Validate()`. Everything else is independent.

Per-project copies had one legitimate use: teams customizing design behavior for their conventions. But this is better handled by config parameters in the central skill than by silent forks that diverge and rot.

Counter-risk (version drift): If a newer DFT adds required fields to a contract, an old skill won't know to produce them and `dft validate` will fail. This is the correct failure mode — the skill needs updating. Since the skills are in one place, you update once. If a project is pinned to an old DFT, the skill can accept `DFT_CONTRACT_VERSION` and adapt its output format (similar to API version negotiation).

### Decision 2: How are front-agent skills imported into Hermes?

**Recommendation**: One-time symlink from the central repo. No per-project setup.

```bash
# Once per machine:
git clone https://github.com/bocacorazon/dft-design-agents.git ~/dft-design-agents
ln -s ~/dft-design-agents/skills/dft-* ~/.hermes/skills/
```

Hermes auto-discovers skills in `~/.hermes/skills/`. Symlinks mean `git pull` in the central repo updates all skills instantly across all projects. No install commands, no per-project copies, no drift.

### Decision 3: How do agents know DFT contract schemas?

**Recommendation**: Call DFT itself. No schema files to maintain.

The skills invoke `dft schema <type>` at runtime to get the current JSON Schema for any contract type. DFT owns the types — it's the authoritative source for what a valid `demand-package.json` or `solution-design.json` looks like.

New DFT subcommands (implementation detail, outside this plan's scope):
- `dft schema <type>` — outputs JSON Schema for `demand-package`, `solution-design`, `test-plan`, `wbs`, `eval-surface-contract`, `lane-assignment`
- `dft validate <type> <file>` — validates a file against the contract's `Validate()` method

The design skill workflow:
```
1. dft schema demand-package → get current schema
2. Generate demand-package.json
3. dft validate demand-package demand-package.json → pass/fail
4. If fail: fix and retry
5. dft schema solution-design → get current schema
6. Generate solution-design.json
7. dft validate solution-design solution-design.json → pass/fail
8. If fail: fix and retry
```

This eliminates schema drift entirely. Skills always validate against the DFT binary that's actually installed. No schema copies, no version mismatches, no maintenance burden.

The `dft/hermes/schemas/` directory from the earlier proposal is not needed. Schemas are runtime artifacts produced by `dft schema`, not files to be curated.

### Decision 4: Skills vs. full agents for the design workflow?

**Recommendation: Skills**, not agent.md files.

The design workflow is a known, repeatable sequence. A skill encapsulates the procedure. The master `dft-design` skill orchestrates sub-skills:

```
dft-design (Hermes skill)
  ├── Phase 1: Intent (autonomous)
  │   ├── Read project context (constitution, codebase, surface contracts)
  │   ├── Self-generate DEMAND: determine what to build
  │   ├── Run ambiguity scan (agent reasons about unclear aspects)
  │   ├── Refine demand into clear specification
  │   ├── Generate acceptance criteria
  │   ├── Run verification (all criteria unambiguous, testable)
  │   └── Output: demand-package.json (validated against schema)
  │
  └── Phase 2: Solution (consumes demand + surface contracts)
      ├── Load demand-package.json
      ├── Load surface contracts (project input, describes available eval surfaces)
      ├── Generate test plan (scenarios with given/when/then)
      ├── Author WBS (work breakdown into specs)
      ├── Assign lanes (which executor per spec, informed by available surfaces)
      ├── Validate solution-design.json against schema
      └── Output: solution-design.json (WBS + lane assignments + test plan)
```

Each sub-phase can be a separate skill for composability, but the master skill orchestrates them end-to-end.

---

## Proposed Repository Structure

```
dft-design-agents/                 # NEW: standalone central repo
├── skills/                        # Hermes skills (each is a dir with SKILL.md)
│   ├── dft-design/
│   │   └── SKILL.md               # Master orchestrator: autonomous Intent → Solution
│   ├── dft-intent/
│   │   └── SKILL.md               # Intent phase: self-generates demand, refines, produces ACs
│   ├── dft-solution/
│   │   └── SKILL.md               # Solution phase: test plan + WBS + lane assignments
│   ├── dft-test-planner/
│   │   └── SKILL.md               # Generates TestPlan from demand package
│   ├── dft-wbs-author/
│   │   └── SKILL.md               # Generates WBS from test plan
│   └── dft-lane-assigner/
│       └── SKILL.md               # Assigns specs to lanes based on surface contracts + model config
├── README.md                      # Setup: git clone + ln -s
└── CONTRIBUTING.md                # How skills work, contract expectations, testing

dft/                               # DFT repo (unchanged)
├── internal/
│   ├── domain/v2/                 # Go contract types (source of truth)
│   │   ├── intent.go              # DemandPackage struct
│   │   ├── solution.go            # SolutionDesign struct
│   │   └── contracts.go           # Validate() methods
│   ├── app/provisioned/github/agents/  # EXISTING: speckit execution agents
│   └── eval/harness/hermes_agent.go    # TO CHANGE: use skill instead of raw prompt
└── cmd/
    └── schema/                    # NEW: dft schema <type> subcommand
        └── main.go                # Outputs JSON Schema for contract types
```

No `dft/hermes/` directory. No schema files to curate. DFT owns the types and exposes them via `dft schema`. The design agents repo owns the skills. One coupling point: the JSON contract.

---

## Artifact Contract Reference

Contracts are defined by DFT's Go types in `internal/domain/v2/`. At runtime, `dft schema <type>` returns the current JSON Schema. The structures below document what the design skills produce. For the authoritative definition, see `dft schema demand-package` and `dft schema solution-design`.

### demand-package.json (Intent output)

Run: `dft schema demand-package` for current schema.
Go type: `internal/domain/v2/intent.go:DemandPackage`

```json
{
  "id": "<demand-package-slug>",
  "title": "<concise title>",
  "raw_demand": "<original demand text>",
  "refined_demand": "<clear unambiguous statement>",
  "acceptance_criteria": [
    {"id": "AC-001", "description": "<observable behavior>", "requirement": "REQ-001"}
  ],
  "assumptions": ["<assumption>"],
  "non_goals": ["<out of scope>"],
  "ambiguities": [
    {"id": "AMB-001", "description": "...", "resolution": "..."}
  ],
  "verified_complete": true,
  "created_at": "<ISO 8601>"
}
```

The `id` field is a slug derived from the demand title (e.g., `extract-design-phase-agents`). This is the `DEMAND_PACKAGE_ID`. It is NOT the DFT `RUN_ID` — those are separate identifiers for separate concerns.

### solution-design.json (Solution output)

Run: `dft schema solution-design` for current schema.
Go type: `internal/domain/v2/solution.go:SolutionDesign`

Note: the `eval_surface_contract` field is NOT authored by the Solution phase. It is consumed from the project's existing surface contracts (passed in as input). The Solution phase produces only the test plan, WBS, and lane assignments.

```json
{
  "demand_package_id": "<demand-package-slug>",
  "test_plan": {
    "demand_package_id": "<demand-package-slug>",
    "requirement_ids": ["REQ-001"],
    "scenarios": [
      {
        "id": "s1",
        "name": "User can authenticate",
        "requirement_ids": ["REQ-001"],
        "description": "Verify auth flow",
        "given": ["User is on login page"],
        "when": ["User enters valid credentials"],
        "then": ["User is redirected to dashboard"]
      }
    ]
  },
  "wbs": {
    "demand_package_id": "<demand-package-slug>",
    "specs": [
      {
        "id": "spec-1",
        "description": "Implement auth endpoint",
        "prompt_path": ".dft/specs/spec-1.md",
        "acceptance_criteria": ["POST /login returns 200 with valid token"]
      }
    ]
  },
  "lane_assignments": [
    {"spec_id": "spec-1", "lane": "speckit", "rationale": "Needs code generation"}
  ],
  "eval_surface_contract": {
    "_note": "consumed from project, not authored here — included for schema completeness"
  },
  "created_at": "<ISO 8601>"
}
```

### DFT validation that downstream code runs

These map to Go `Validate()` methods:
- `DemandPackage.Validate()` — id, title, raw_demand, refined_demand, acceptance_criteria, verified_complete
- `SolutionDesign.Validate()` — demand_package_id, test_plan scenarios (id, name, then), WBS (specs with id, acceptance_criteria, description/prompt_path). Note: `eval_surface_contract` is consumed from project input, not validated as authored output.

### Surface contracts: input, not output

The design phase reads surface contracts from the project. These describe what eval surfaces exist (CLI, HTTP API, database, etc.) and their adapter families. The Solution phase uses them to:
- Assign specs to appropriate lanes based on available surfaces
- Ensure the WBS targets surfaces that actually exist
- Ground the test plan in real, verifiable endpoints

The surface contracts themselves are authored during project setup (by a human or a separate surface-discovery tool), not by the design phase.

---

## Skill Design: dft-design (Master Orchestrator)

**Location**: `dft-design-agents/skills/dft-design/SKILL.md`

**Trigger**: `hermes -w chat --skill dft-design` or invoked from within Hermes

**Inputs** (all flow in from DFT, none generated by the design phase):
- `RUN_ID`: DFT execution run identifier — generated by `dft submit` or `dft build`. Used for artifact paths and audit trails, NOT as the demand package id.
- `WORKSPACE`: working directory (where to write output files)
- `SURFACE_CONTRACTS`: path to the project's existing eval surface contract(s) — these describe the available eval surfaces the design phase must target

**Autonomously generated by the design phase:**
- `DEMAND`: the design phase inspects the project (constitution, codebase, existing surfaces, TODO state) and determines what feature/demand to work on. It self-generates both `raw_demand` and `refined_demand`.
- `DEMAND_PACKAGE_ID`: a human-readable slug derived from the demand title (e.g., `extract-design-phase-agents`, `add-oauth2-support`). This is the primary identifier for the demand package and is used as `id` in demand-package.json and `demand_package_id` in solution-design.json.

**Outputs**:
- `demand-package.json` in workspace root (self-contained, id = DEMAND_PACKAGE_ID slug)
- `solution-design.json` in workspace root (references DEMAND_PACKAGE_ID; WBS, lane assignments, test plan)

**Workflow**:

```
1. Read project context: constitution.md, existing codebase structure, surface contracts
2. dft schema demand-package → get current contract schema
3. Autonomously generate DEMAND:
   → Determine what feature/bug/improvement to build
   → Produce raw_demand and refined_demand
   → Derive DEMAND_PACKAGE_ID slug from title (e.g., "extract-design-phase-agents")
4. Load dft-intent skill with self-generated DEMAND
   → Run ambiguity scan, refine, generate acceptance criteria
   → Output: demand-package.json (id = DEMAND_PACKAGE_ID)
5. dft validate demand-package demand-package.json
   → if fail: fix and retry (max 2 attempts)
6. dft schema solution-design → get current contract schema
7. Load dft-solution skill with:
   → demand-package.json (freshly generated)
   → Surface contracts (input, already known)
   → Produces test plan, WBS, lane assignments
   → Output: solution-design.json (demand_package_id = DEMAND_PACKAGE_ID)
8. dft validate solution-design solution-design.json
   → if fail: fix and retry (max 2 attempts)
9. Report paths, RUN_ID, DEMAND_PACKAGE_ID to user
```

The skill orchestrates sub-skills. Each sub-skill is independently usable for targeted phases.

### Key design principle: the design phase is autonomous

The design phase does not wait for a human to say "build X." It reads the project's constitution, surfaces, and current state, then autonomously determines what feature to work on. The demand is an output artifact, not an input. RUN_ID is an execution-context identifier from DFT (for audit/artifact paths), while DEMAND_PACKAGE_ID is a meaningful slug derived from the demand title — these are separate identifiers for separate concerns.

---

## How the DFT Harness Changes

**Current** (`internal/eval/harness/hermes_agent.go`):
```go
prompt := "You are the architecture team..." + demand  // 72-line monolithic prompt
args := []string{"-w", "chat", "-q", prompt, "-Q"}
cmd := exec.CommandContext(ctx, "hermes", args...)
```

**Proposed**:
```go
// DFT passes RUN_ID and workspace; the design skill generates demand autonomously
args := []string{"-w", "chat", "--skill", "dft-design", "-Q"}
cmd := exec.CommandContext(ctx, "hermes", args...)
cmd.Env = append(os.Environ(),
    "HERMES_WORKTREE="+workspaceDir,
    "DFT_RUN_ID="+runID,
    "DFT_SURFACE_CONTRACTS="+surfaceContractsPath,  // project's existing eval surfaces
)

// Alternatively, pass a short prompt that loads the skill:
prompt := fmt.Sprintf(
    "Load the dft-design skill and run it with RUN_ID=%q. Surface contracts are at %s. Write outputs to %s.",
    runID, surfaceContractsPath, designDir,
)
```

The harness becomes a thin shim — no design logic, just invocation.

---

## Open Design Questions

### Q1: Granularity of sub-skills

Should we have:
- **Coarse**: Just `dft-design` (one skill, all phases inlined)
- **Medium**: `dft-intent` + `dft-solution` (two skills, surface contracts passed as input)
- **Fine**: `dft-intent`, `dft-test-planner`, `dft-wbs-author`, `dft-lane-assigner` (four skills)

**Recommendation: Medium** for v1. Two skills is the right granularity:
- `dft-intent` autonomously generates a demand and produces a validated demand package
- `dft-solution` consumes the demand package and surface contracts, produces test plan + WBS + lane assignments
- Each is independently testable and reusable
- Fine-grained skills can be extracted later if sub-phases need independent invocation
- Note: `dft-surface-contractor` is NOT a design-phase skill — surface contracts are project-level inputs authored during setup

### Q3: Skill installation trigger

When does the symlink get created?
- Manual: `ln -s ~/dft-design-agents/skills/dft-* ~/.hermes/skills/`
- During `dft init` (automatic)
- Git post-clone hook in the design-agents repo

**Recommendation: Manual one-time setup with README instructions.** It's two commands: `git clone` + `ln -s`. A `dft init` hook could do it automatically, but that couples DFT to the design-agents repo and adds complexity for minimal gain. Document it in the README and move on.

### Q4: What about the existing agent.md files?

The existing speckit.*.agent.md files at `internal/app/provisioned/github/agents/` are execution agents for GitHub Copilot. They are NOT replaced by this design. The front agents (Intent + Solution) are new skills; execution agents remain as-is until a separate effort addresses them.

---

## Risks & Tradeoffs

| Risk | Mitigation |
|------|-----------|
| DFT contract changes break skills | Skills call `dft validate` after writing; DFT's own `Validate()` catches mismatches at runtime. If a contract adds required fields, the skill fails validation visibly rather than producing invalid output silently. Fix the skill once in the central repo and all projects benefit. |
| Central repo + old DFT version mismatch | Skills accept `DFT_CONTRACT_VERSION` parameter to adapt output format. DFT's `json.Unmarshal` ignores unknown fields (Go's default), so additive schema changes are backward-compatible. Breaking changes are flagged by `dft validate` at design time. |
| Hermes doesn't support `--skill` flag | Fallback: use `Load the dft-design skill and run it...` as prompt; Hermes skills auto-load from conversation context |
| Over-engineering for v1 | Start with 2 skills (dft-intent, dft-solution), expand granularity only when proven necessary |
| Skills diverge across forks | Central repo with single source of truth eliminates this. Custom project behavior is handled by config parameters, not forks. |

---

## Implementation Sequence (for future execution)

1. **Add `dft schema` subcommand**: DFT CLI exposes contract schemas as JSON. Generates from Go types (code-gen at build time or reflection at startup).
2. **Add `dft validate` subcommand**: CLI wrapper around existing `Validate()` methods for on-demand validation.
3. **Create `dft-design-agents` repo**: Standalone repo with skill directory structure.
4. **Create `dft-intent` skill**: SKILL.md with full Intent workflow, calls `dft schema demand-package` + `dft validate`.
5. **Create `dft-solution` skill**: SKILL.md with full Solution workflow, calls `dft schema solution-design` + `dft validate`.
6. **Create `dft-design` skill**: Master orchestrator that chains Intent → Solution with validation gates.
7. **Symlink skills into Hermes**: `ln -s ~/dft-design-agents/skills/dft-* ~/.hermes/skills/`
8. **Update `hermes_agent.go`**: Replace monolithic prompt with skill invocation.
9. **Test end-to-end**: Run `dft submit` in a test project, verify the design phase produces valid artifacts that flow into the execution layer.

---

## Summary

The front-agent environment lives in a standalone `dft-design-agents/` repo, not inside DFT:
- **`skills/`** — Hermes skills for Intent and Solution phases
- Single symlink: `ln -s ~/dft-design-agents/skills/dft-* ~/.hermes/skills/`
- One `git pull` updates all projects

DFT exposes its contracts at runtime via `dft schema <type>` and `dft validate <type> <file>`. No schema files to curate, no copies to drift. The skills ask DFT what it expects and produce output that matches.

Front agents are Hermes skills, not opaque agent.md files. DFT's harness becomes a thin invocation layer — no design logic in Go. The coupling surface is the JSON contract, which is DFT's public API.

**Data flow summary**:
```
DFT harness                         Design phase (Hermes skills)
───────────                         ─────────────────────────────
RUN_ID ───────────────────────────→ used for artifact paths/audit
surface contracts ────────────────→ consumed by Solution phase
                                    │
                                    ├─ dft-intent:
                                    │    Self-generates DEMAND
                                    │    Derives DEMAND_PACKAGE_ID slug
                                    │    → demand-package.json (id = slug)
                                    │
                                    └─ dft-solution: reads demand + surfaces
                                       → solution-design.json
                                         (demand_package_id = slug)

demand-package.json  ←───────────── written to workspace
solution-design.json ←───────────── written to workspace
```

**Identifier separation**:
- `RUN_ID` — DFT execution context (from `dft submit` / `dft build`). Used for artifact directories, audit trails, step tracking.
- `DEMAND_PACKAGE_ID` — human-readable slug (e.g., `extract-design-phase-agents`). The demand package's identity, used as the `id` / `demand_package_id` field in JSON artifacts.

The existing speckit.*.agent.md files for execution remain untouched. This design covers only the front (design) agents.
