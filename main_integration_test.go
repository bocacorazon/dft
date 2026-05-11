package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/store"
)

type fakeSubmitter struct {
	runID string
	fs    *store.Filesystem
}

func (s fakeSubmitter) Submit(_ context.Context, flowFile string) (string, error) {
	if err := s.fs.StartRun(s.runID, flowFile); err != nil {
		return "", err
	}
	if err := s.fs.MarkRunSucceeded(s.runID); err != nil {
		return "", err
	}
	return s.runID, nil
}

func (s fakeSubmitter) Resume(_ context.Context, runID string) (string, error) {
	if err := s.fs.MarkRunRunning(runID); err != nil {
		return "", err
	}
	if err := s.fs.MarkRunSucceeded(runID); err != nil {
		return "", err
	}
	return runID, nil
}

func TestSubmitThenStatus(t *testing.T) {
	tmp := t.TempDir()
	flowFile := filepath.Join(tmp, "flow.yaml")
	filesystem := store.NewFilesystem(filepath.Join(tmp, "runs"))
	engine := fakeSubmitter{runID: "run-integration-1", fs: filesystem}

	var submitOut bytes.Buffer
	var submitErr bytes.Buffer
	submitCode := runWithDeps([]string{"submit", flowFile}, &submitOut, &submitErr, engine, filesystem)
	if submitCode != 0 {
		t.Fatalf("submit code = %d, stderr=%q", submitCode, submitErr.String())
	}

	runID := strings.TrimSpace(strings.TrimPrefix(submitOut.String(), "run-id:"))
	if runID == "" {
		t.Fatalf("expected run-id output, got %q", submitOut.String())
	}

	var statusOut bytes.Buffer
	var statusErr bytes.Buffer
	statusCode := runWithDeps([]string{"status", runID}, &statusOut, &statusErr, engine, filesystem)
	if statusCode != 0 {
		t.Fatalf("status code = %d, stderr=%q", statusCode, statusErr.String())
	}
	if !strings.Contains(statusOut.String(), "state: succeeded") {
		t.Fatalf("expected succeeded status, got %q", statusOut.String())
	}
}

func TestResumeCommand(t *testing.T) {
	tmp := t.TempDir()
	flowFile := filepath.Join(tmp, "flow.yaml")
	filesystem := store.NewFilesystem(filepath.Join(tmp, "runs"))
	engine := fakeSubmitter{runID: "run-integration-2", fs: filesystem}

	var submitOut bytes.Buffer
	var submitErr bytes.Buffer
	if code := runWithDeps([]string{"submit", flowFile}, &submitOut, &submitErr, engine, filesystem); code != 0 {
		t.Fatalf("submit failed: %s", submitErr.String())
	}
	runID := strings.TrimSpace(strings.TrimPrefix(submitOut.String(), "run-id:"))

	var resumeOut bytes.Buffer
	var resumeErr bytes.Buffer
	if code := runWithDeps([]string{"resume", runID}, &resumeOut, &resumeErr, engine, filesystem); code != 0 {
		t.Fatalf("resume failed: %s", resumeErr.String())
	}
	if !strings.Contains(resumeOut.String(), runID) {
		t.Fatalf("unexpected resume output: %q", resumeOut.String())
	}
}
