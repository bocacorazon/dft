---
description: Generate a demand-package YAML from free-form feature intent.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding.

## Task

Convert the input into a demand-package and output **YAML only**.

Required top-level keys:

- `id`
- `title`
- `description`
- `acceptance_criteria` (array, min 5)
- `constraints` (array)
- `assumptions` (array)
- `out_of_scope` (array)
- `references` (array)

## Rules

1. Keep scope to one increment.
2. Acceptance criteria must be testable and observable.
3. Make assumptions explicit; do not ask follow-up questions.
4. No prose or markdown code fences outside YAML.

