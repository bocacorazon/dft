---
description: Assign execution lanes for specs in a WBS.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding.

## Task

Output **YAML only**:

- `demand_package_id`
- `assignments` (array)
  - `spec_id`
  - `lane`
  - `rationale`

## Bootstrap policy

- Default lane is `spec`.
- Use `manual` only when automation is clearly unsuitable.

## Rules

1. Do not change spec IDs.
2. Keep rationale concise and concrete.
3. No prose or markdown code fences outside YAML.

