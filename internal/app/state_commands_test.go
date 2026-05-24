package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/state"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
	"github.com/bocacorazon/dft/internal/orchestration"
)

func TestStatusInspectCancelAndResumeCommands(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("DFT_RUN_ID", "state-run")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"submit", "--adapter", "stub", "--dry-run", "--dogfood", "Track dogfood runs"}, &stdout, &stderr); code != 0 {
		t.Fatalf("submit returned %d\nstderr: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Fatalf("status returned %d\nstderr: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "state-run") || !strings.Contains(got, "succeeded") {
		t.Fatalf("status output = %q, want run and status", got)
	}
	if got := stdout.String(); !strings.Contains(got, "lane/001-track-dogfood-runs") {
		t.Fatalf("status output = %q, want lane summary", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "state.db")); err != nil {
		t.Fatalf("state.db missing: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"inspect", "state-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect returned %d\nstderr: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "evaluation.json") || !strings.Contains(got, "next-demand-package.json") {
		t.Fatalf("inspect output = %q, want artifacts", got)
	}
	if got := stdout.String(); !strings.Contains(got, "lane/001-track-dogfood-runs") {
		t.Fatalf("inspect output = %q, want lane detail", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"cancel", "state-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("cancel returned %d\nstderr: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"resume", "state-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("resume returned %d\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "state-run") {
		t.Fatalf("resume output = %q, want run id", stdout.String())
	}
}

func TestResumeCommandResumesSingleSpecLaneFromArtifacts(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	runID := "resume-run"
	spec := domain.SpecRef{
		ID:                 "001-resume-smoke",
		Description:        "Resume smoke",
		AcceptanceCriteria: []string{"specify writes spec.md"},
	}
	worktree := specWorktreeForRun(runID, spec.ID)
	definition, err := orchestration.LoadSpecKitLane(".", spec, worktree)
	if err != nil {
		t.Fatalf("LoadSpecKitLane returned error: %v", err)
	}
	runner := flow.Runner{
		Agent:        agentstub.Adapter{},
		Dispatcher:   agentstub.Adapter{},
		ArtifactRoot: ".",
		RunID:        runID,
		Verifier:     verify.Checker{RootDir: "."},
	}
	if _, err := runner.Execute(context.Background(), definition); err == nil {
		t.Fatal("Execute returned nil error, want pause at review-spec gate")
	}
	wbsContent, err := json.MarshalIndent(domain.WBS{
		DemandPackageID: runID,
		Specs:           []domain.SpecRef{spec},
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal WBS: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(".dft", "runs", runID, "design"), 0o755); err != nil {
		t.Fatalf("mkdir design dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".dft", "runs", runID, "design", "wbs.json"), append(wbsContent, '\n'), 0o644); err != nil {
		t.Fatalf("write WBS: %v", err)
	}
	sqlStore, err := state.OpenSQLiteStore(filepath.Join(".dft", "state.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer sqlStore.Close()
	if err := saveRunState(state.JSONStore{RootDir: "."}, sqlStore, domain.RunManifest{
		ID:        runID,
		Status:    domain.RunFailed,
		Adapter:   "stub",
		RawDemand: spec.Description,
	}); err != nil {
		t.Fatalf("saveRunState returned error: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"resume", runID}, &stdout, &stderr); code != 0 {
		t.Fatalf("resume returned %d\nstderr: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, runID) || !strings.Contains(got, "plan") {
		t.Fatalf("resume output = %q, want run id and resumed stage", got)
	}
	manifest, err := loadRunManifest(runID, state.JSONStore{RootDir: "."})
	if err != nil {
		t.Fatalf("loadRunManifest returned error: %v", err)
	}
	if manifest.Status != domain.RunSucceeded {
		t.Fatalf("manifest status = %q, want %q", manifest.Status, domain.RunSucceeded)
	}
	if _, err := os.Stat(filepath.Join(worktree.WorktreePath, "specs", spec.ID, "tasks.md")); err != nil {
		t.Fatalf("tasks artifact missing after resume: %v", err)
	}
}

func TestLoadResumableSpecForRunSelectsActiveSpecFromArtifacts(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	runID := "resume-run"
	specs := []domain.SpecRef{
		{ID: "001-active", Description: "Active", AcceptanceCriteria: []string{"one"}},
		{ID: "002-idle", Description: "Idle", AcceptanceCriteria: []string{"two"}},
	}
	content, err := json.MarshalIndent(domain.WBS{DemandPackageID: runID, Specs: specs}, "", "  ")
	if err != nil {
		t.Fatalf("marshal WBS: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(".dft", "runs", runID, "design"), 0o755); err != nil {
		t.Fatalf("mkdir design dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".dft", "runs", runID, "design", "wbs.json"), append(content, '\n'), 0o644); err != nil {
		t.Fatalf("write WBS: %v", err)
	}
	for path, data := range map[string]string{
		filepath.Join(".specify", "templates", "spec-template.md"):                                "# Spec Template\nTODO\n",
		filepath.Join(".specify", "templates", "plan-template.md"):                                "# Plan Template\nTODO\n",
		filepath.Join(".specify", "templates", "tasks-template.md"):                               "# Tasks Template\n- [ ] TODO\n",
		filepath.Join(".dft", "worktrees", runID, "001-active", "specs", "001-active", "spec.md"): "# Concrete spec\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir parent dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(".dft", "worktrees", runID, "001-active"), 0o755); err != nil {
		t.Fatalf("mkdir active worktree dir: %v", err)
	}

	spec, err := loadResumableSpecForRun(runID)
	if err != nil {
		t.Fatalf("loadResumableSpecForRun returned error: %v", err)
	}
	if spec.ID != "001-active" {
		t.Fatalf("spec id = %q, want 001-active", spec.ID)
	}
}
