package cli

import (
	"context"
	"fmt"
	"io"

	dfterrors "github.com/bocacorazon/dft/internal/errors"
	"github.com/bocacorazon/dft/internal/store"
)

type Submitter interface {
	Submit(ctx context.Context, flowFile string) (string, error)
	Resume(ctx context.Context, runID string) (string, error)
}

type StatusStore interface {
	GetRun(runID string) (store.RunRecord, error)
}

func RunSubmit(args []string, stdout io.Writer, stderr io.Writer, engine Submitter) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: dft submit <flow-file>")
		return 1
	}

	runID, err := engine.Submit(context.Background(), args[0])
	if err != nil {
		if runID != "" {
			fmt.Fprintf(stderr, "run-id: %s\n", runID)
		}
		fmt.Fprintf(stderr, "submit failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "run-id: %s\n", runID)
	return 0
}

func RunResume(args []string, stdout io.Writer, stderr io.Writer, engine Submitter) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: dft resume <run-id>")
		return 1
	}

	runID, err := engine.Resume(context.Background(), args[0])
	if err != nil {
		fmt.Fprintf(stderr, "resume failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "run-id: %s\n", runID)
	return 0
}

func RunStatus(args []string, stdout io.Writer, stderr io.Writer, statusStore StatusStore) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: dft status <run-id>")
		return 1
	}

	record, err := statusStore.GetRun(args[0])
	if err != nil {
		if _, ok := err.(dfterrors.RunNotFoundError); ok {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprintf(stderr, "status failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "run-id: %s\n", record.RunID)
	fmt.Fprintf(stdout, "state: %s\n", record.State)
	if record.Error != "" {
		fmt.Fprintf(stdout, "error: %s\n", record.Error)
	}
	return 0
}
