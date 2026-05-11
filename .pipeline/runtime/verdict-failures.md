# MVR verdict failures (iteration 1)

## Build failures

1. `go build ./...` fails: `main.go` imports packages that do not exist in the
   repository:
   - `github.com/bocacorazon/dft/internal/cli`
   - `github.com/bocacorazon/dft/internal/copilot`
   - `github.com/bocacorazon/dft/internal/runner`
   - `github.com/bocacorazon/dft/internal/store`

2. `go mod tidy` fails because the above internal packages are missing.

## Acceptance criteria gaps

1. `dft submit <flow-file>` behavior is not implemented.
2. `dft status <run-id>` behavior is not implemented.
3. No file-backed RunStore implementation exists under `.dft/runs`.
4. No agent adapter implementation exists for `copilot -p ... --agent ...`.
5. No flow parser implementation exists for agent-only steps.
6. No capture/export_as plumbing exists for downstream context.

## Fix target

Implement only the missing packages and functions required to satisfy these
criteria, with runnable `go build` / `go test` outcomes.

