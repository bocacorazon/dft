package orchestration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConcreteMarkdownFileRejectsBareHeading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte("# Spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	ready, err := concreteMarkdownFile(path, "")
	if err != nil {
		t.Fatalf("concreteMarkdownFile error: %v", err)
	}
	if ready {
		t.Fatalf("bare-heading markdown should not be concrete")
	}
}

func TestConcreteMarkdownFileRejectsWhitespaceTweakedTemplate(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "spec-template.md")
	if err := os.WriteFile(templatePath, []byte("# Spec\n\nDescribe the feature here.\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	// Same content as the template but with extra blank lines and trailing spaces;
	// a byte checksum would differ, but it is still just the template.
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte("# Spec   \n\n\nDescribe the feature here.  \n\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	ready, err := concreteMarkdownFile(path, templatePath)
	if err != nil {
		t.Fatalf("concreteMarkdownFile error: %v", err)
	}
	if ready {
		t.Fatalf("whitespace-tweaked template copy should not be concrete")
	}
}

func TestConcreteMarkdownFileAcceptsSubstantiveContent(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "spec-template.md")
	if err := os.WriteFile(templatePath, []byte("# Spec\n\nDescribe the feature here.\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte("# Spec\n\nEnable real submit with verifiable acceptance criteria.\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	ready, err := concreteMarkdownFile(path, templatePath)
	if err != nil {
		t.Fatalf("concreteMarkdownFile error: %v", err)
	}
	if !ready {
		t.Fatalf("substantive non-template markdown should be concrete")
	}
}

func TestFileHasTaskCheckbox(t *testing.T) {
	dir := t.TempDir()
	withTasks := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(withTasks, []byte("# Tasks\n\n- [ ] T001 Do the thing\n"), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
	headingOnly := filepath.Join(dir, "tasks-empty.md")
	if err := os.WriteFile(headingOnly, []byte("# Tasks\n\nSome prose but no checklist items.\n"), 0o644); err != nil {
		t.Fatalf("write tasks-empty: %v", err)
	}

	if ok, err := fileHasTaskCheckbox(withTasks); err != nil || !ok {
		t.Fatalf("expected checkbox detected (ok=%v err=%v)", ok, err)
	}
	if ok, err := fileHasTaskCheckbox(headingOnly); err != nil || ok {
		t.Fatalf("expected no checkbox detected (ok=%v err=%v)", ok, err)
	}
}

func TestDecideSpecKitLaneResumeBlocksOnBareHeadingSpec(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	// Bare heading: previously passed as concrete, now must block at specify.
	fixture.writeFile(t, filepath.Join(fixture.featureDir, "spec.md"), "# Feature Spec\n")

	definition := BuildBaseSpecKitFlow(fixture.spec)
	decision, err := DecideSpecKitLaneResume(definition, fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("DecideSpecKitLaneResume returned error: %v", err)
	}
	if decision.ResumeStepID != "specify" {
		t.Fatalf("resume step = %q, want specify", decision.ResumeStepID)
	}
	if decision.Completed {
		t.Fatalf("bare-heading spec should not yield a completed lane")
	}
}

func TestDecideSpecKitLaneResumeBlocksOnTasksWithoutCheckbox(t *testing.T) {
	fixture := newSpecKitArtifactFixture(t)
	fixture.writeSpec(t)
	fixture.writePlan(t)
	// Tasks file has substantive prose but no checklist items.
	fixture.writeTasks(t, "# Tasks\n\nWe will implement the increment carefully.\n")

	definition := BuildBaseSpecKitFlow(fixture.spec)
	decision, err := DecideSpecKitLaneResume(definition, fixture.root, fixture.runID, fixture.spec, fixture.worktree)
	if err != nil {
		t.Fatalf("DecideSpecKitLaneResume returned error: %v", err)
	}
	if decision.ResumeStepID != "tasks" {
		t.Fatalf("resume step = %q, want tasks", decision.ResumeStepID)
	}
}
