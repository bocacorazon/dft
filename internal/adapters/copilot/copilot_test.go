package copilot

import (
	"context"
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
	if err := os.WriteFile(binary, []byte("#!/usr/bin/env sh\nprintf '{\"ok\":true,\"agent\":\"%s\"}\\n' \"$2\"\nprintf 'warn\\n' >&2\n"), 0o755); err != nil {
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
	for _, name := range []string{"stdout.txt", "stderr.txt", "prompt.md"} {
		if _, err := os.Stat(filepath.Join(root, "transcripts", "dft-intake.agent.md", name)); err != nil {
			t.Fatalf("expected transcript %s: %v", name, err)
		}
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
