package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type HermesAgent struct {
	Profile string // Optional hermes profile (e.g. "dft-eval")
	Model   string // Optional model override
}

func (h *HermesAgent) Name() string { return "hermes" }

func (h *HermesAgent) Design(ctx context.Context, increment string, workspaceDir string, designDir string) error {
	// The design skill is autonomous — it generates its own increment from project context.
	// We pass RUN_ID for audit trails and the workspace/surface paths.
	prompt := fmt.Sprintf(
		"Load and run the dft-design skill. DFT_RUN_ID=%s. DFT_WORKSPACE=%s. Surface contracts are in .dft/surfaces/. Write increment-package.json and solution-design.json to %s.",
		h.runID(), workspaceDir, designDir,
	)

	args := []string{"chat", "-q", prompt, "-Q"}
	if h.Profile != "" {
		args = append(args, "--profile", h.Profile)
	}
	if h.Model != "" {
		args = append(args, "-m", h.Model)
	}

	cmd := exec.CommandContext(ctx, "hermes", args...)
	cmd.Env = append(os.Environ(),
		"HERMES_WORKTREE="+workspaceDir,
		"DFT_RUN_ID="+h.runID(),
		"DFT_WORKSPACE="+workspaceDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hermes design failed (exit code %v): %s", err, string(out))
	}
	return nil
}

func (h *HermesAgent) runID() string {
	// RUN_ID is set by the caller (eval harness) before invoking Design.
	// If unset, generate a timestamp-based fallback.
	return fmt.Sprintf("run-%d", time.Now().Unix())
}
