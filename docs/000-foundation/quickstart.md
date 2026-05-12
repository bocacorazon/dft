# dft v1 Quickstart

Use `dft init` once in a target repository to provision `.dft/` assets, Copilot agent files, context files, lane/flow scaffolding, and the sqlite-backed run store.

```sh
dft init
DFT_RUN_ID=my-increment dft submit --adapter stub --dry-run --dogfood "Describe the demand package"
dft status
dft inspect my-increment
```

The default stub adapter is deterministic and suitable for smoke tests. Use `--adapter copilot` with a configured GitHub Copilot CLI when running real agent-backed intake and lane steps.

For each increment, dft creates an increment branch from the repository default branch, creates spec branches from the increment branch, runs eval, writes final review artifacts, and only merges after eval and review pass. Remote-only steps such as PR creation/check/merge write audit records under `.dft/runs/<run-id>/remote/` instead of creating local commits.
