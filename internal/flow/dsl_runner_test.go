package flow

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
)

func TestRunnerExecutesStagesSetupToolsFunctionsAfterAndVerification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tool fixture uses POSIX sh")
	}
	root := t.TempDir()
	script := filepath.Join(root, "write-file.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env sh\nprintf '%s' \"$1\" > \"$2\"\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runner := Runner{
		ArtifactRoot: root,
		RunID:        "run-123",
		Verifier:     verify.Checker{RootDir: root},
	}
	result, err := runner.Execute(context.Background(), Definition{
		Stages: []Stage{{
			ID: "build",
			Setup: []Step{{
				ID:       "set-output",
				Type:     StepFunction,
				Function: "set_var",
				Args: map[string]string{
					"name":  "output",
					"value": "result.txt",
				},
			}},
			Steps: []Step{{
				ID:      "write",
				Type:    StepTool,
				Command: []string{script, "hello", filepath.Join(root, "result.txt")},
			}},
			After: []Step{{
				ID:      "after",
				Type:    StepTool,
				Command: []string{script, "after", filepath.Join(root, "after.txt")},
			}},
			Verify: []domain.Check{
				{ID: "result", Kind: domain.CheckGrepMatches, Args: []string{"result.txt", "hello"}},
				{ID: "after", Kind: domain.CheckFileExists, Args: []string{"after.txt"}},
			},
		}},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Vars["output"] != "result.txt" {
		t.Fatalf("vars = %#v, want output var", result.Vars)
	}
	if len(result.Verification) != 1 || result.Verification[0].Status != domain.VerdictPass {
		t.Fatalf("verification = %#v, want one passing result", result.Verification)
	}
}

func TestRunnerDoesNotRunStageAfterWhenMainStepFails(t *testing.T) {
	root := t.TempDir()
	runner := Runner{ArtifactRoot: root, RunID: "run-123"}

	_, err := runner.Execute(context.Background(), Definition{
		Stages: []Stage{{
			ID: "build",
			Steps: []Step{{
				ID:      "missing-tool",
				Type:    StepTool,
				Command: []string{"definitely-not-a-real-dft-command"},
			}},
			After: []Step{{
				ID:      "after",
				Type:    StepTool,
				Command: []string{"sh", "-c", "touch should-not-exist"},
			}},
		}},
	})

	if err == nil {
		t.Fatal("Execute returned nil error, want tool failure")
	}
	if _, statErr := os.Stat(filepath.Join(root, "should-not-exist")); !os.IsNotExist(statErr) {
		t.Fatalf("after step ran despite failure: %v", statErr)
	}
}
