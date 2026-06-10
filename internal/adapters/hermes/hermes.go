package hermes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/ports"
)

// Adapter invokes Hermes CLI as a subprocess.
type Adapter struct {
	Binary        string
	Cwd           string
	TranscriptDir string
	Timeout       time.Duration
	Env           []string
}

func (a Adapter) Invoke(ctx context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	binary := a.Binary
	if binary == "" {
		binary = "hermes"
	}
	if a.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.Timeout)
		defer cancel()
	}

	baseDir := a.Cwd
	if baseDir == "" {
		baseDir = "."
	}
	cmdDir := baseDir
	if request.Cwd != "" {
		if filepath.IsAbs(request.Cwd) {
			cmdDir = request.Cwd
		} else {
			cmdDir = filepath.Join(baseDir, request.Cwd)
		}
	}
	absCmdDir, err := filepath.Abs(cmdDir)
	if err != nil {
		return ports.AgentResponse{}, fmt.Errorf("resolve hermes cwd: %w", err)
	}

	// Prepare arguments
	args := []string{
		"chat", "-q", request.Prompt,
	}
	if request.Model != "" {
		args = append(args, "-m", request.Model)
	}

	if request.AllowTools {
		args = append(args, "--yolo")
	}

	// Prepend skills if agentName is given, mapping agent name to skill name.

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = absCmdDir
	cmd.Env = append(os.Environ(), a.Env...)
	for k, v := range request.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	usage := ports.AgentUsage{
		Model:      request.Model,
		DurationMs: time.Since(start).Milliseconds(),
		ExitCode:   exitCodeFromError(err),
	}
	if err != nil {
		return ports.AgentResponse{Usage: usage}, fmt.Errorf("hermes agent %q failed: %w: %s", request.AgentName, err, stderr.String())
	}

	return ports.AgentResponse{
		Raw:   strings.TrimSpace(stdout.String()),
		Usage: usage,
	}, nil
}

// exitCodeFromError extracts the process exit code from a command error,
// returning 0 on success and -1 when the code is unavailable.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func (a Adapter) DispatchCommand(ctx context.Context, request ports.CommandRequest) (ports.CommandResponse, error) {
	binary := a.Binary
	if binary == "" {
		binary = "hermes"
	}
	if request.Command == "" {
		return ports.CommandResponse{}, fmt.Errorf("command name is required")
	}
	if a.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.Timeout)
		defer cancel()
	}

	baseDir := a.Cwd
	if baseDir == "" {
		baseDir = "."
	}
	cmdDir := baseDir
	if request.Cwd != "" {
		if filepath.IsAbs(request.Cwd) {
			cmdDir = request.Cwd
		} else {
			cmdDir = filepath.Join(baseDir, request.Cwd)
		}
	}
	absCmdDir, err := filepath.Abs(cmdDir)
	if err != nil {
		return ports.CommandResponse{}, fmt.Errorf("resolve hermes cwd: %w", err)
	}

	// We pass the exact command action to hermes via chat
	prompt := fmt.Sprintf("Execute command: %s\nInput:\n%s", request.Command, request.Input)

	args := []string{
		"chat", "-q", prompt,
	}

	if request.AllowTools {
		args = append(args, "--yolo")
	}
	if request.Model != "" {
		args = append(args, "-m", request.Model)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = absCmdDir
	cmd.Env = append(os.Environ(), a.Env...)
	for k, v := range request.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return ports.CommandResponse{}, fmt.Errorf("hermes command %q failed: %w: %s", request.Command, err, stderr.String())
	}

	return ports.CommandResponse{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}, nil
}
