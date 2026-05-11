---
description: Draft a conventional commit message from current changes.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding.

## Task

Return plain text only:

1. First line: `type(scope): summary`
2. Optional body lines with intent/impact

## Rules

1. Reflect only the provided/current diff.
2. Be specific and concise.
3. Do not include co-author trailers.

