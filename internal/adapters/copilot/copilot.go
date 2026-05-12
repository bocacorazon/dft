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

	cmdDir := a.Cwd
	if request.Cwd != "" {
		cmdDir = request.Cwd
	}
	args := []string{
		"-C", cmdDir,
		"--agent", copilotAgentName(request.AgentName),
		"-p", request.Prompt,
		"--allow-all",
		"--no-ask-user",
		"--autopilot",
		"-s",
		"--output-format", "text",
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = cmdDir
	cmd.Env = append(os.Environ(), a.Env...)
	for key, value := range request.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
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
