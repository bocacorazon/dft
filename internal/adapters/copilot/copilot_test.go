package copilot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bocacorazon/dft/internal/ports"
)

func TestAdapterInvokesCopilotBinaryAndCapturesTranscript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-specific")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "fake-copilot")
	if err := os.WriteFile(binary, []byte(`#!/usr/bin/env sh
agent=""
prompt=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --agent) shift; agent="$1" ;;
    -p|--prompt) shift; prompt="$1" ;;
  esac
  shift
done
printf '{"ok":true,"agent":"%s","prompt":"%s"}\n' "$agent" "$prompt"
printf 'warn\n' >&2
`), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	adapter := Adapter{
		Binary:        binary,
		Cwd:           root,
		TranscriptDir: filepath.Join(root, "transcripts"),
		Timeout:       time.Second,
	}
	response, err := adapter.Invoke(context.Background(), ports.AgentRequest{
		AgentName: "dft-intake.agent.md",
		Prompt:    "Normalize increment",
		RunID:     "run-123",
		StepID:    "intent",
		Attempt:   1,
	})

	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if !strings.Contains(response.Raw, `"ok":true`) {
		t.Fatalf("raw response = %q, want fake JSON", response.Raw)
	}
	if response.Usage.ExitCode != 0 {
		t.Fatalf("usage exit code = %d, want 0", response.Usage.ExitCode)
	}
	transcriptStep := filepath.Join(root, "transcripts", "intent", "attempt-1")
	for _, name := range []string{"stdout.txt", "stderr.txt", "prompt.md", "argv.json"} {
		if _, err := os.Stat(filepath.Join(transcriptStep, name)); err != nil {
			t.Fatalf("expected transcript %s: %v", name, err)
		}
	}
	rawArgv, err := os.ReadFile(filepath.Join(transcriptStep, "argv.json"))
	if err != nil {
		t.Fatalf("read argv transcript: %v", err)
	}
	var argv []string
	if err := json.Unmarshal(rawArgv, &argv); err != nil {
		t.Fatalf("argv transcript invalid JSON: %v\n%s", err, rawArgv)
	}
	if !containsSequence(argv, "--agent", "dft-intake") || !containsSequence(argv, "-p", "Normalize increment") {
		t.Fatalf("argv = %#v, want --agent and -p prompt", argv)
	}
	if containsValue(argv, "--allow-all") {
		t.Fatalf("argv = %#v, structured agent should not allow tools by default", argv)
	}
}

func TestAdapterAllowsToolsOnlyWhenRequested(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-specific")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "fake-copilot")
	if err := os.WriteFile(binary, []byte("#!/usr/bin/env sh\nprintf '{\"ok\":true}\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	adapter := Adapter{Binary: binary, Cwd: root, TranscriptDir: filepath.Join(root, "transcripts"), Timeout: time.Second}

	if _, err := adapter.Invoke(context.Background(), ports.AgentRequest{AgentName: "speckit.implement.agent.md", Prompt: "Implement", RunID: "run-123", StepID: "implement", Attempt: 1, AllowTools: true}); err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	rawArgv, err := os.ReadFile(filepath.Join(root, "transcripts", "implement", "attempt-1", "argv.json"))
	if err != nil {
		t.Fatalf("read argv transcript: %v", err)
	}
	var argv []string
	if err := json.Unmarshal(rawArgv, &argv); err != nil {
		t.Fatalf("argv transcript invalid JSON: %v\n%s", err, rawArgv)
	}
	if !containsValue(argv, "--allow-all") || !containsValue(argv, "--autopilot") {
		t.Fatalf("argv = %#v, tool-enabled agent should allow tools and autopilot", argv)
	}
}

func TestAdapterReturnsContextForNonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-specific")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "fake-copilot")
	if err := os.WriteFile(binary, []byte("#!/usr/bin/env sh\nprintf 'bad news\\n' >&2\nexit 7\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	adapter := Adapter{Binary: binary, Cwd: root, Timeout: time.Second}
	response, err := adapter.Invoke(context.Background(), ports.AgentRequest{AgentName: "dft-intake.agent.md", Prompt: "x", RunID: "run-123"})

	if err == nil {
		t.Fatal("Invoke returned nil error, want non-zero exit error")
	}
	if !strings.Contains(err.Error(), "bad news") {
		t.Fatalf("error = %v, want stderr context", err)
	}
	if response.Usage.ExitCode != 7 {
		t.Fatalf("usage exit code = %d, want 7", response.Usage.ExitCode)
	}
}

func TestDispatchCommandPassesModelAndAgentName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-specific")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "fake-copilot")
	if err := os.WriteFile(binary, []byte(`#!/usr/bin/env sh
agent=""
prompt=""
model=""
argv="$*"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --agent) shift; agent="$1" ;;
    -p|--prompt) shift; prompt="$1" ;;
    --model) shift; model="$1" ;;
  esac
  shift
done
printf '{"agent":"%s","prompt":"%s","model":"%s","argv":"%s"}\n' "$agent" "$prompt" "$model" "$argv"
`), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	adapter := Adapter{Binary: binary, Cwd: root, Timeout: time.Second}

	response, err := adapter.DispatchCommand(context.Background(), ports.CommandRequest{
		Command: "speckit.specify",
		Input:   "Build auth",
		Model:   "gpt-5-mini",
		RunID:   "run-123",
	})
	if err != nil {
		t.Fatalf("DispatchCommand returned error: %v", err)
	}
	if !strings.Contains(response.Stdout, `"agent":"speckit.specify"`) {
		t.Fatalf("stdout = %q, want command agent name", response.Stdout)
	}
	if !strings.Contains(response.Stdout, `"model":"gpt-5-mini"`) {
		t.Fatalf("stdout = %q, want model flag forwarded", response.Stdout)
	}
	if !strings.Contains(response.Stdout, `"argv":"-p Build auth --agent speckit.specify --no-ask-user --model gpt-5-mini"`) {
		t.Fatalf("stdout = %q, want command dispatch argv", response.Stdout)
	}
}

func containsSequence(values []string, first string, second string) bool {
	for i := 0; i+1 < len(values); i++ {
		if values[i] == first && values[i+1] == second {
			return true
		}
	}
	return false
}

func containsValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
