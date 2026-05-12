package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInitAndSyncProvisionDftAssets(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("init returned %d\nstderr: %s", code, stderr.String())
	}
	for _, path := range []string{
		filepath.Join(".dft", "agents", "dft-intake.agent.md"),
		filepath.Join(".dft", "flows", "spec-lane.json"),
		filepath.Join(".dft", "context", "constitution.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected provisioned asset %s: %v", path, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"sync"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync returned %d\nstderr: %s", code, stderr.String())
	}
}
