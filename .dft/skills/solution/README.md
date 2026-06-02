# dft Solution Phase

These skills handle the Solution phase — turning a verified demand package into a complete solution design ready for build.

## Workflow

```
DemandPackage (verified_complete: true)
    ↓
test-planner: produces test plan with Gherkin scenarios
    ↓
wbs-author: produces WBS with specs and acceptance criteria
    ↓
surface-contract-author: declares eval surfaces for each spec
    ↓
lane-assignment: assigns each spec to speckit/direct/stub lane
    ↓
Assembly: Hermes combines all artifacts into solution-design.json
    ↓
Ready for dft build
```

## Output

`.dft/runs/<run-id>/design/solution-design.json` — the contract handed to `dft build`.

## Usage from Hermes

```
User: "Design the solution for the bookmark API"

Hermes (loads test-planner):
  → Produces 12 test scenarios covering 7 acceptance criteria
  "Test plan: 12 scenarios. Coverage: 7/7 ACs."

Hermes (loads wbs-author):
  → Produces WBS with 4 specs
  "WBS: 4 specs. All test scenarios mapped."

Hermes (loads surface-contract-author):
  → Declares 2 eval surfaces (HTTP API + CLI)
  "2 eval surfaces declared."

Hermes (loads lane-assignment):
  → Assigns lanes: 3 speckit, 1 direct
  "Lanes: 3 speckit, 1 direct."

Hermes (assembles solution-design.json):
  "Solution design complete. 4 specs, 12 test scenarios, 2 eval surfaces.
   Save and proceed to build?"

User: "Save it"
  → solution-design.json written
```

## Schema

The `SolutionDesign` type is defined in `internal/domain/v2/solution.go`. It uses:
- `WBS` and `SpecRef` from `internal/domain/wbs.go` (existing)
- `LaneAssignment` from `internal/domain/wbs.go` (existing)
- `EvalSurfaceContract` from `internal/domain/eval.go` (existing)
- `TestPlan` from `internal/domain/v2/solution.go` (new)
