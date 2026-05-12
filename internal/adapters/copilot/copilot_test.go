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
		Prompt:    "Normalize demand",
		RunID:     "run-123",
	})

	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if !strings.Contains(response.Raw, `"ok":true`) {
		t.Fatalf("raw response = %q, want fake JSON", response.Raw)
	}
	for _, name := range []string{"stdout.txt", "stderr.txt", "prompt.md", "argv.json"} {
		if _, err := os.Stat(filepath.Join(root, "transcripts", "dft-intake.agent.md", name)); err != nil {
			t.Fatalf("expected transcript %s: %v", name, err)
		}
	}
	rawArgv, err := os.ReadFile(filepath.Join(root, "transcripts", "dft-intake.agent.md", "argv.json"))
	if err != nil {
		t.Fatalf("read argv transcript: %v", err)
	}
	var argv []string
	if err := json.Unmarshal(rawArgv, &argv); err != nil {
		t.Fatalf("argv transcript invalid JSON: %v\n%s", err, rawArgv)
	}
	if !containsSequence(argv, "--agent", "dft-intake") || !containsSequence(argv, "-p", "Normalize demand") {
		t.Fatalf("argv = %#v, want --agent and -p prompt", argv)
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
	_, err := adapter.Invoke(context.Background(), ports.AgentRequest{AgentName: "dft-intake.agent.md", Prompt: "x", RunID: "run-123"})

	if err == nil {
		t.Fatal("Invoke returned nil error, want non-zero exit error")
	}
	if !strings.Contains(err.Error(), "bad news") {
		t.Fatalf("error = %v, want stderr context", err)
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
