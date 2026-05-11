package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/bocacorazon/dft/internal/runner"
	"github.com/bocacorazon/dft/internal/store"
)

type fakeAdapter struct {
	output string
	err    error
}

func (a fakeAdapter) RunAgent(_ context.Context, _ string, _ string, _ string) (string, error) {
	return a.output, a.err
}

func TestRun_NoArgs_PrintsUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	filesystem := store.NewFilesystem(t.TempDir())
	engine := runner.NewEngine(filesystem, fakeAdapter{output: "ok"})

	code := runWithDeps([]string{}, &stdout, &stderr, engine, filesystem)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Fatalf("expected usage in stderr")
	}
}

func TestRun_UnknownCommand_ReturnsError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	filesystem := store.NewFilesystem(t.TempDir())
	engine := runner.NewEngine(filesystem, fakeAdapter{output: "ok"})

	code := runWithDeps([]string{"wat"}, &stdout, &stderr, engine, filesystem)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Fatalf("expected unknown command error")
	}
}
