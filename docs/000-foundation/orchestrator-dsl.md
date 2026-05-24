# Orchestrator YAML Executor Plan

> Follow-on plan for dft's YAML workflow executor and the separation between DSL
> execution and worktree orchestration.

## Current state

The corrected direction is:

- dft owns a **YAML DSL**
- the executor reads that YAML and runs it
- LLM-invoking steps shell out directly to Copilot in the validated
  `skrunner`/Speckit-runner style
- worktree and branch orchestration remain separate from workflow definition

## Problem

The immediate repair problem is that dft drifted into a JSON flow path when the
intended design was a YAML DSL executor. The executor needs to be corrected so
that:

- YAML is the external workflow format
- the base Speckit lane is represented in YAML
- orchestration loads that YAML rather than synthesizing a JSON-backed flow

Once that is corrected, the remaining dft-specific lifecycle behavior still
hardcoded in `MacroOrchestrator` can be addressed:

- eval-plan authoring
- evaluation execution
- remediation retries after failed evaluation
- final review
- remediation retries after blocked review
- final blocked-review artifact handling

That means there are two layers of work:

1. correct the executor path back to YAML-first execution
2. later move broader dft workflow behavior into workflow data

## Proposed direction

The proposal is:

1. correct the executor so it reads **dft YAML workflows**
2. encode the proven base Speckit flow in YAML
3. preserve direct Copilot subprocess execution for LLM steps
4. only then expand higher-level dft workflow behavior around that base lane

### Immediate implementation slice

1. Add YAML loading and validation to `internal/flow`.
2. Encode the base Speckit lane as YAML in `.dft/flows/spec-lane.yaml`.
3. Make orchestration load that YAML and bind runtime inputs into it.
4. Keep LLM-invoking command steps aligned with the current direct Copilot
   subprocess pattern.

### Later expansion after the YAML correction

1. Add a higher-level dft workflow definition that reuses the base Speckit lane.
2. Represent evaluation planning declaratively.
3. Represent evaluation execution declaratively.
4. Represent remediation retries declaratively.
5. Represent final review declaratively.
6. Add the smallest workflow-engine extension needed to express those dft-only
   actions as data.

### Explicit non-goals for this phase

- designing a new JSON DSL
- keeping JSON as the primary external workflow format
- using an uber-agent model for workflow execution
- worktree creation or cleanup in workflow data
- branch merge policy in workflow data
- increment/spec branch topology changes
- moving `CompleteIncrement` / mergeback behavior into the workflow definition

Those stay outside the workflow definition until the later worktree/orchestration
phase.

## Implementation outline

1. Define the dft YAML workflow schema and internal Go mapping.
2. Build YAML parsing and validation.
3. Replace the `.json` flow path with `.yaml`.
4. Provision the YAML workflow into target repositories.
5. Keep worktree begin/complete and mergeback control in `MacroOrchestrator`
   until the later worktree refactor.

## Boundary for the next phase

This plan is specifically about restoring the intended **YAML-first executor**
and then later moving broader dft workflow behavior into workflow data. It is
not yet the phase for moving **worktree or branch orchestration** into workflow
data.

## Retrospective: how the design drift happened

This section is here as a future guardrail. The previous implementation drift
was not mainly a lack-of-context failure; it was a failure to keep the user's
actual architecture constraints locked in while iterating.

### What went wrong

1. `workflow.yml` and `skrunner` were treated as references, not invariants.
   - `workflow.yml` was shown to establish that the external DSL is **YAML**.
   - `skrunner` was shown to establish that LLM steps should **shell out
     directly to Copilot** rather than use an uber-agent model.
   - Treating them as inspiration instead of hard constraints let the design
     drift without an explicit decision to do so.

2. Sequencing was misread as architecture change.
   - The intended sequence was:
     1. prove the base Speckit flow
     2. encode it in YAML
     3. add dft-specific stages in YAML
     4. reattach worktree orchestration later
   - The wrong implementation interpreted that as permission to hardcode the
     base flow first and defer YAML entirely, which silently changed the target
     architecture.

3. A local optimization hardened into the design.
   - “Prove behavior first” was a reasonable tactical move.
   - But once the temporary `.json` flow path was provisioned and described as
     “the same flow definition the runtime uses,” the temporary shortcut became
     a de facto architectural decision.

4. Planning continued on top of the wrong base.
   - Once the JSON flow path existed, later planning began expanding dft stages
     around that path.
   - That moved the conversation away from the more basic question:
     *is the executor even reading the correct external DSL?*

5. Prompt/artifact calibration overshadowed format fidelity.
   - The prompt simplifications and artifact gates were useful.
   - But they solved a different problem than the one the architecture discussion
     was trying to solve. The core ask was about DSL format and execution
     boundaries, not just phase correctness.

### What should have been scrutinized earlier

- Any proposal using words like **temporary**, **first**, **for now**, or
  **prove it before encoding it** should trigger one explicit check:
  *does this still preserve YAML as the external source of truth?*
- Any claim like **“the runtime and provisioning use the same definition”**
  should trigger:
  *the same definition in which file, and in which format?*
- If the intended design is “YAML executor,” but there is no tracked YAML file
  for the flow, that is an immediate warning sign that the implementation has
  drifted.

### Practical guardrails for future sessions

Before implementation starts, write down the non-negotiable invariants in plain
language and keep checking them during the session:

1. **External workflow format:** YAML
2. **LLM execution boundary:** direct subprocess shell-out to Copilot
3. **Orchestration boundary:** worktree/branch lifecycle stays outside workflow
   definition

If an intermediate proposal violates one of those, then it is not a step toward
the desired design. It is a different design and should be treated as such
immediately.
