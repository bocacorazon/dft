Build increment 1 (Minimum Viable Runner) for dft in Go.

Must include:
- `dft submit <flow-file>` and `dft status <run-id>` CLI commands.
- Minimal YAML flow parsing for `agent` steps only.
- Copilot subprocess adapter (`copilot -p ... --agent ...`).
- File-backed run metadata under `.dft/runs/<run-id>/`.
- Step audit artifacts under `.dft/runs/<run-id>/<step-id>/`.
- `capture: true` and `export_as` support for downstream step templating.

Scope limits:
- local execution only
- no sqlite yet
- no worktrees/commit orchestration yet

