# dft User Manual

`dft` is a headless workflow engine for spec-driven software production. You run
it inside a Git repository to provision managed assets, submit a demand, and
inspect the resulting artifacts, review output, and merge state.

## What dft manages

| Term | Meaning |
| --- | --- |
| Run | One execution attempt, identified by a run ID. |
| Demand package | The normalized JSON version of the user's request. |
| Increment branch | The integration branch for one run, named `increment/<run-id>`. |
| Spec branch/worktree | The per-spec isolated workspace under `.dft/worktrees/<run-id>/<spec-id>`. |
| Run artifacts | Durable outputs under `.dft/runs/<run-id>/`. |

## Prerequisites

- A Git repository with an initial commit and a known default branch.
- Go if you are building `dft` from source.
- For real agent-backed runs: GitHub Copilot CLI available as `copilot` (or
  passed with `--copilot-binary`).
- For GitHub-backed end-to-end runs: `gh` authenticated and available on `PATH`.
- For smoke tests: no agent account is required; use `--adapter stub`.

## Overall developer workflow

1. Build the CLI.

   ```sh
   go build -o bin/dft ./cmd/dft
   ```

2. Provision dft into the target repository once.

   ```sh
   ./bin/dft init
   git add .
   git commit -m "provision dft assets"
   ```

3. Refresh managed assets when the toolkit changes.

   ```sh
   ./bin/dft sync --force
   ```

4. Start with a dry-run smoke test. This writes run artifacts but skips local
   git mutations and engine-owned commits.

   ```sh
   DFT_RUN_ID=smoke-run ./bin/dft submit --adapter stub --dry-run --full \
     "Build a small CLI that prints its version"
   ```

5. When the smoke path looks good, run the real flow with Copilot.

   ```sh
   DFT_RUN_ID=feature-001 ./bin/dft submit --adapter copilot \
     --copilot-binary copilot --full --agent-timeout 45m --eval-retries 1 \
     --hold-increment \
     "Build a minimal Go CLI named democtl with --version"
   ```

6. Monitor and inspect the run.

   ```sh
   ./bin/dft status
   ./bin/dft inspect feature-001
   ```

7. If a run was interrupted, resume it from the next incomplete stage.

   ```sh
   ./bin/dft resume feature-001
   ```

8. If you need to stop tracking a run, mark it cancelled and keep the artifacts.

   ```sh
   ./bin/dft cancel feature-001
   ```

## What a full run does

When you use `dft submit --full`, dft drives a full increment:

1. Intake turns raw demand into `intent/demand-package.json`.
2. dft creates an increment branch from the repository default branch.
3. Design authoring produces the WBS, lane assignments, and eval surfaces.
4. For each spec, dft runs the Speckit lane: `specify`, `plan`, `tasks`,
   `analyze`, `implement`, review, and mergeback.
5. Evaluation authors an eval plan and executes deterministic checks.
6. Final review runs against the increment diff.
7. The increment merges back to the default branch unless
   `--hold-increment`/`--no-merge` is set.

`--dogfood` is a superset of `--full`: it runs the full process and also writes
feedback artifacts such as `next-demand-package.json` for the next increment.

## CLI usage

`dft`, `dft help`, and `dft --help` all print the top-level command list.
Subcommands do not implement their own `--help` handling, so use the table
below as the reference for current usage instead of commands like
`dft submit --help`.

| Command | Usage | What it does |
| --- | --- | --- |
| `help` | `dft help` | Prints the top-level help text. |
| `init` | `dft init [--force]` | Provisions managed `.dft/`, `.github/agents/`, `.specify/`, and related assets in the current repository. |
| `sync` | `dft sync [--force]` | Refreshes managed assets using the provisioning manifest. |
| `submit` | `dft submit [flags] <demand text>` | Creates a run. With `--full` or `--dogfood`, it executes the macro workflow. |
| `status` | `dft status` | Lists known runs and, when available, lane summaries for each spec. |
| `inspect` | `dft inspect <run-id>` | Prints files under `.dft/runs/<run-id>/`, then durable step, inbox, and lane status. |
| `cancel` | `dft cancel <run-id>` | Updates the stored run status to `cancelled`. Artifacts stay on disk. |
| `resume` | `dft resume <run-id>` | Reconstructs the active spec from artifacts and resumes from the next resumable stage. |

### `submit` flags

| Flag | Meaning |
| --- | --- |
| `--adapter stub\|copilot` | Selects the agent adapter. The default is `stub`. |
| `--copilot-binary <path>` | Overrides the Copilot executable used by the `copilot` adapter. |
| `--dry-run` | Writes artifacts but skips local git mutations and engine-owned commits. |
| `--full` / `--execute` | Runs the full macro process instead of intake only. |
| `--dogfood` | Runs the full process plus the dogfood feedback loop. |
| `--hold-increment` / `--no-merge` | Keeps the increment branch instead of merging it back to the default branch. |
| `--eval-retries <n>` | Sets the maximum eval remediation retries. |
| `--agent-timeout <duration>` | Sets the per-agent timeout, for example `30m` or `45m`. |
| `DFT_RUN_ID` | Environment variable for a stable run ID; otherwise dft generates `run-YYYYMMDD-HHMMSS`. |

## Reading command output

- `status` prints one line per run as `run-id<TAB>status<TAB>raw-demand`, then
  optional `lane/<spec-id>` lines with the latest successful stage, blocking
  stage, and resume recommendation.
- `inspect` first prints artifact-relative paths under `.dft/runs/<run-id>/`,
  then durable lines such as `state/steps/...`, `inbox/...`, and
  `lane/<spec-id>`.

## Key files and directories

| Path | Contents |
| --- | --- |
| `.dft/state.db` | Durable run, job, step, and inbox state. |
| `.dft/runs/<run-id>/intent/` | Intake prompt, stdout capture, and `demand-package.json`. |
| `.dft/runs/<run-id>/design/` | WBS, lane assignments, and eval surfaces. |
| `.dft/runs/<run-id>/eval/` | Readiness, eval plan, evaluation result, and evidence. |
| `.dft/runs/<run-id>/review/` | Final review output. |
| `.dft/runs/<run-id>/transcripts/` | Copilot adapter transcripts: prompt, argv, stdout, stderr. |
| `.dft/worktrees/<run-id>/<spec-id>/` | Per-spec worktree and generated Spec Kit artifacts. |

## Working on dft itself

When you are changing `dft`, the repository constitution expects test-first Go
development. Keep `go test ./...` green, use `go vet ./...` as the additional
static check, and rebuild the CLI with `go build ./cmd/dft` before manual smoke
or end-to-end runs.

For a real GitHub/Copilot-backed operator check, use `scripts/real-e2e.sh`. It
builds `dft`, provisions a test repository, runs `submit --adapter copilot
--full`, and validates the generated software and run artifacts.
