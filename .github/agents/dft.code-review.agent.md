---
description: Review code changes for correctness, security, and logic defects.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding.

## Task

Review the provided diff/context and return **JSON only**:

```json
{
  "critical": 0,
  "important": 0,
  "minor": 0,
  "findings": [
    {
      "severity": "critical|important|minor",
      "file": "path/to/file",
      "line": 1,
      "description": "issue",
      "suggestion": "fix"
    }
  ]
}
```

## Rules

1. Focus only on bugs, security issues, logic errors, and missing error handling.
2. Ignore style and formatting-only comments.
3. If no issues, return zero counts and an empty `findings` array.

