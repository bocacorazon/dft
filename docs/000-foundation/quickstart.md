# dft Quickstart

Use `dft init` once in a target repository to provision `.dft/` assets, flows,
agent files, context files, and the sqlite-backed run store.

For the longer operator guide, see [../user-manual.md](../user-manual.md). For
upstream design artifacts, see [../design-phase/README.md](../design-phase/README.md).

## 1. Initialize the repository

```sh
dft init
```

## 2. Execute one frozen spec

```sh
dft build \
  --prompt-file specs/001-auth/prompt.md \
  --workflow single-spec \
  --feature-slug 001-auth \
  --increment-branch increment/auth-platform \
  --models config/models.json
```

## 3. Execute a WBS submission

```sh
dft submit \
  --wbs design/wbs.json \
  --workflow single-spec \
  --models config/models.json
```

## 4. Inspect progress

```sh
dft status
dft inspect <run-id>
```

The default stub adapter is deterministic and suitable for smoke tests. Use
`--adapter copilot` with a configured GitHub Copilot CLI when running real
agent-backed execution flows.
