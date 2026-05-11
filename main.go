package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bocacorazon/dft/internal/cli"
	"github.com/bocacorazon/dft/internal/copilot"
	"github.com/bocacorazon/dft/internal/runner"
	"github.com/bocacorazon/dft/internal/store"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	runStore, err := store.NewSQLite(".dft/state.db", ".dft/runs")
	if err != nil {
		fmt.Fprintf(stderr, "failed to initialize sqlite store: %v\n", err)
		return 1
	}
	engine := runner.NewEngine(runStore, copilot.NewAdapter())
	return runWithDeps(args, stdout, stderr, engine, runStore)
}

func runWithDeps(args []string, stdout io.Writer, stderr io.Writer, engine cli.Submitter, statusStore cli.StatusStore) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	switch args[0] {
	case "submit":
		return cli.RunSubmit(args[1:], stdout, stderr, engine)
	case "resume":
		return cli.RunResume(args[1:], stdout, stderr, engine)
	case "status":
		return cli.RunStatus(args[1:], stdout, stderr, statusStore)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  dft submit <flow-file>")
	fmt.Fprintln(out, "  dft resume <run-id>")
	fmt.Fprintln(out, "  dft status <run-id>")
}
