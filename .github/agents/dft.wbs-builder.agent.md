---
description: Build a dependency-ordered WBS YAML from a demand-package.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding.

## Task

Read a demand-package and produce **YAML only** with:

- `demand_package_id`
- `specs` (array of objects)
  - `id`, `title`, `description`, `lane`, `depends_on`, `acceptance_criteria`

## Rules

1. Produce a valid DAG: no cyclic dependencies.
2. Keep each spec independently completable.
3. Use lane `spec` by default unless input strongly indicates `manual`.
4. No prose or markdown code fences outside YAML.

