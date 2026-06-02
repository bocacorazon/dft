# dft Intent Phase

These skills handle the Intent phase of dft — turning raw demand into a verified demand package.

## Workflow

```
User: "I need a CLI tool that converts markdown to PDF with table support"
    ↓
ambiguity-scanner: scans for vague terms, missing context, conflicts
    ↓ (user resolves ambiguities in conversation)
demand-refiner: produces refined demand + acceptance criteria + assumptions + non-goals
    ↓
ac-verifier: checks AC coverage, atomicity, testability
    ↓ (if gaps: loops back to refiner)
demand-package.json written with verified_complete: true
    ↓
Ready for Solution phase
```

## Output

`.dft/runs/<run-id>/intent/demand-package.json` — the contract handed to the Solution phase.

## Usage from Hermes

Hermes loads these skills automatically when the user asks to design or refine a feature. The user never runs `dft intent` — they just describe what they want, and Hermes drives the agents.

Example:
```
User: "Help me design a REST API for managing bookmarks"

Hermes (loads ambiguity-scanner):
  "I found 3 ambiguities: 'fast' isn't defined, the auth model isn't specified,
   and it's unclear if bookmarks can be organized into folders. Want to clarify?"

User: "Sub-200ms responses, API key auth, flat list no folders"

Hermes (loads demand-refiner):
  → Produces refined demand with 7 acceptance criteria
  "Demand package drafted: 'Bookmark REST API'. 7 ACs, 3 assumptions, 2 non-goals."

Hermes (loads ac-verifier):
  "AC coverage: 7/7 requirements covered. All ACs are testable. Verified complete.
   Save the demand package?"

User: "Save it"
  → demand-package.json written to .dft/runs/<run-id>/intent/
```
