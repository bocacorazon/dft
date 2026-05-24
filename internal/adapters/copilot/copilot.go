package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/ports"
)

// Adapter invokes GitHub Copilot CLI as a subprocess.
type Adapter struct {
	Binary        string
	Cwd           string
	TranscriptDir string
	Timeout       time.Duration
	Env           []string
}

// DispatchCommand invokes a named workflow command through Copilot agent mode.
func (a Adapter) DispatchCommand(ctx context.Context, request ports.CommandRequest) (ports.CommandResponse, error) {
	binary := a.Binary
	if binary == "" {
		binary = "copilot"
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
		return ports.CommandResponse{}, fmt.Errorf("resolve copilot cwd: %w", err)
	}
	stem := request.Command
	if strings.HasPrefix(stem, "speckit.") {
		stem = strings.TrimPrefix(stem, "speckit.")
	}
	agentName := "speckit." + stem
	args := []string{
		"-p", request.Input,
		"--agent", agentName,
		"--no-ask-user",
	}
	if request.AllowTools {
		args = append(args, "--yolo")
	}
	if request.Model != "" {
		args = append(args, "--model", request.Model)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = absCmdDir
	cmd.Env = append(os.Environ(), a.Env...)
	for key, value := range request.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return ports.CommandResponse{}, fmt.Errorf("copilot command %q failed: %w: %s", request.Command, err, detail)
	}
	return ports.CommandResponse{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}, nil
}

// Invoke runs the configured Copilot binary with explicit argv and captures output.
func (a Adapter) Invoke(ctx context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	binary := a.Binary
	if binary == "" {
		binary = "copilot"
	}
	if request.AgentName == "" {
		return ports.AgentResponse{}, fmt.Errorf("agent name is required")
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
		return ports.AgentResponse{}, fmt.Errorf("resolve copilot cwd: %w", err)
	}
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return ports.AgentResponse{}, fmt.Errorf("resolve copilot base cwd: %w", err)
	}
	args := []string{
		"-C", absCmdDir,
		"--agent", copilotAgentName(request.AgentName),
		"-p", request.Prompt,
		"--no-ask-user",
		"-s",
		"--output-format", "text",
	}
	if request.AllowTools {
		args = append(args, "--allow-all", "--autopilot")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = absBaseDir
	cmd.Env = append(os.Environ(), a.Env...)
	for key, value := range request.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if writeErr := a.writeTranscript(request, args, stdout.Bytes(), stderr.Bytes()); writeErr != nil && err == nil {
		return ports.AgentResponse{}, writeErr
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return ports.AgentResponse{}, fmt.Errorf("copilot agent %q failed: %w: %s", request.AgentName, err, detail)
	}
	return ports.AgentResponse{Raw: stdout.String()}, nil
}

func copilotAgentName(name string) string {
	base := filepath.Base(name)
	return strings.TrimSuffix(base, ".agent.md")
}

func (a Adapter) writeTranscript(request ports.AgentRequest, argv []string, stdout []byte, stderr []byte) error {
	if a.TranscriptDir == "" {
		return nil
	}
	dir := filepath.Join(a.TranscriptDir, request.AgentName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create transcript directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(request.Prompt), 0o644); err != nil {
		return fmt.Errorf("write prompt transcript: %w", err)
	}
	argvContent, err := json.MarshalIndent(argv, "", "  ")
	if err != nil {
		return fmt.Errorf("encode argv transcript: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "argv.json"), append(argvContent, '\n'), 0o644); err != nil {
		return fmt.Errorf("write argv transcript: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stdout.txt"), stdout, 0o644); err != nil {
		return fmt.Errorf("write stdout transcript: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stderr.txt"), stderr, 0o644); err != nil {
		return fmt.Errorf("write stderr transcript: %w", err)
	}
	return nil
}
