package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySpeckitImplementArtifactsAcceptsPartialProgress(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "specs", "001-auth")
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("mkdir feature dir: %v", err)
	}
	tasksFile := filepath.Join(featureDir, "tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [X] Done task\n- [ ] Remaining task\n"), 0o644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}

	artifacts, err := verifySpeckitCommandArtifacts(root, Step{
		CommandName: "speckit.implement",
		Cwd:         root,
		Env: map[string]string{
			"SPECIFY_FEATURE_DIRECTORY": "specs/001-auth",
		},
	})
	if err != nil {
		t.Fatalf("verifySpeckitCommandArtifacts returned error: %v", err)
	}
	if artifacts["completed_tasks"] != false {
		t.Fatalf("completed_tasks = %#v, want false when tasks remain", artifacts["completed_tasks"])
	}
	if artifacts["task_progress"] != true {
		t.Fatalf("task_progress = %#v, want true", artifacts["task_progress"])
	}
}

func TestReadTaskChecklistStatusCountsCompletedAndIncompleteTasks(t *testing.T) {
	root := t.TempDir()
	tasksFile := filepath.Join(root, "tasks.md")
	if err := os.WriteFile(tasksFile, []byte("- [X] Done\n- [x] Also done\n- [ ] Remaining\n"), 0o644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}

	status, err := ReadTaskChecklistStatus(tasksFile)
	if err != nil {
		t.Fatalf("ReadTaskChecklistStatus returned error: %v", err)
	}
	if status.Total != 3 || status.Completed != 2 || status.Incomplete != 1 {
		t.Fatalf("status = %#v, want total=3 completed=2 incomplete=1", status)
	}
	if status.AllCompleted() {
		t.Fatal("AllCompleted = true, want false with one remaining task")
	}
}
