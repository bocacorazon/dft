package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		filepath.Join(".dft", "agents", "dft-fix-planner.agent.md"),
		filepath.Join(".dft", "lanes", "spec.json"),
		filepath.Join(".dft", "flows", "spec-lane.json"),
		filepath.Join(".dft", "context", "constitution.md"),
		filepath.Join(".dft", "context", "project.md"),
		filepath.Join(".dft", "inbox", ".gitkeep"),
		filepath.Join(".github", "agents", "dft-intake.agent.md"),
		filepath.Join(".github", "agents", "speckit.implement.agent.md"),
		filepath.Join(".github", "copilot", "agents", "dft-intake.agent.md"),
		filepath.Join(".specify", "memory", "constitution.md"),
		filepath.Join(".dft", "provisioning-manifest.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected provisioned asset %s: %v", path, err)
		}
	}
	assertProvisioningManifest(t, filepath.Join(".dft", "provisioning-manifest.json"))

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"sync"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync returned %d\nstderr: %s", code, stderr.String())
	}
}

func TestSyncDoesNotClobberUserOwnedFiles(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(".dft", "context"), 0o755); err != nil {
		t.Fatalf("create context dir: %v", err)
	}
	userContent := "user-owned constitution\n"
	path := filepath.Join(".dft", "context", "constitution.md")
	if err := os.WriteFile(path, []byte(userContent), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"sync"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync returned %d\nstderr: %s", code, stderr.String())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context file: %v", err)
	}
	if string(content) != userContent {
		t.Fatalf("sync clobbered user-owned file: %q", content)
	}
}

func TestSyncUpdatesManagedFiles(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("init returned %d\nstderr: %s", code, stderr.String())
	}
	path := filepath.Join(".dft", "agents", "dft-intake.agent.md")
	if err := os.WriteFile(path, []byte(managedHeader+"\nstale\n"), 0o644); err != nil {
		t.Fatalf("write managed file: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"sync"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync returned %d\nstderr: %s", code, stderr.String())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read managed file: %v", err)
	}
	if strings.Contains(string(content), "stale") {
		t.Fatalf("sync did not update managed file: %q", content)
	}
}

func assertProvisioningManifest(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read provisioning manifest: %v", err)
	}
	var manifest struct {
		Assets []provisionedHash `json:"assets"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("provisioning manifest invalid JSON: %v\n%s", err, content)
	}
	if len(manifest.Assets) == 0 {
		t.Fatalf("manifest assets = 0, want provisioned hashes")
	}
	for _, asset := range manifest.Assets {
		if asset.Path == "" || len(asset.SHA256) != 64 {
			t.Fatalf("invalid manifest asset: %#v", asset)
		}
	}
}
