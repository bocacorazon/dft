package copilot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Adapter struct {
	binary string
	flags  []string
}

func NewAdapter() *Adapter {
	flags := strings.Fields(os.Getenv("DFT_COPILOT_FLAGS"))
	if len(flags) == 0 {
		flags = []string{"--allow-all", "-s", "--no-ask-user", "--autopilot"}
	}

	binary := os.Getenv("DFT_COPILOT_BIN")
	if binary == "" {
		binary = "copilot"
	}

	return &Adapter{
		binary: binary,
		flags:  flags,
	}
}

func (a *Adapter) RunAgent(ctx context.Context, workDir string, agent string, prompt string) (string, error) {
	args := make([]string, 0, len(a.flags)+4)
	args = append(args, a.flags...)
	args = append(args, "-p", prompt, "--agent", agent)

	cmd := exec.CommandContext(ctx, a.binary, args...)
	cmd.Dir = workDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("copilot agent run failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}
