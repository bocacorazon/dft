package integration_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCLIHelp(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/dft", "--help")
	cmd.Dir = "../.."

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/dft --help failed: %v\n%s", err, output)
	}
	if got := string(output); !strings.Contains(got, "Usage:") || !strings.Contains(got, "submit") {
		t.Fatalf("help output missing expected content:\n%s", got)
	}
}
