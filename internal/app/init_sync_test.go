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
		".gitignore",
		filepath.Join(".dft", "agents", "dft-intake.agent.md"),
		filepath.Join(".dft", "agents", "dft-fix-planner.agent.md"),
		filepath.Join(".dft", "agents", "dft-mergeback.agent.md"),
		filepath.Join(".dft", "lanes", "spec.json"),
		filepath.Join(".dft", "flows", "spec-lane.yaml"),
		filepath.Join(".dft", "context", "constitution.md"),
		filepath.Join(".dft", "context", "project.md"),
		filepath.Join(".dft", "inbox", ".gitkeep"),
		filepath.Join(".github", "agents", "dft-intake.agent.md"),
		filepath.Join(".github", "agents", "speckit.implement.agent.md"),
		filepath.Join(".github", "copilot", "agents", "dft-intake.agent.md"),
		filepath.Join(".github", "copilot", "agents", "speckit.implement.agent.md"),
		filepath.Join(".specify", "memory", "constitution.md"),
		filepath.Join(".dft", "provisioning-manifest.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected provisioned asset %s: %v", path, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(".dft", "flows", "spec-lane.yaml"))
	if err != nil {
		t.Fatalf("read spec lane flow: %v", err)
	}
	if !strings.Contains(string(content), "command: speckit.specify") || !strings.Contains(string(content), "command: speckit.analyze") || !strings.Contains(string(content), "function: gh_issues_from_findings") {
		t.Fatalf("spec lane flow does not provision the enhanced workflow stages:\n%s", content)
	}
	if !strings.Contains(string(content), `args: "{{ inputs.specify_input }}"`) || !strings.Contains(string(content), `SPECIFY_FEATURE_DIRECTORY: "{{ inputs.feature_directory }}"`) || !strings.Contains(string(content), `target: "{{ inputs.increment_branch }}"`) {
		t.Fatalf("spec lane flow does not preserve declarative spec input binding:\n%s", content)
	}
	gitignore, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gitignore), ".dft/runs/") || !strings.Contains(string(gitignore), ".dft/inbox/*") {
		t.Fatalf(".gitignore missing dft runtime ignores:\n%s", gitignore)
	}
	specifyContent, err := os.ReadFile(filepath.Join(".github", "agents", "speckit.specify.agent.md"))
	if err != nil {
		t.Fatalf("read speckit specify agent: %v", err)
	}
	if !strings.Contains(string(specifyContent), "## Outline") {
		t.Fatalf("speckit specify agent was not provisioned with real Spec Kit content")
	}
	copilotSpecifyContent, err := os.ReadFile(filepath.Join(".github", "copilot", "agents", "speckit.specify.agent.md"))
	if err != nil {
		t.Fatalf("read copilot speckit specify agent: %v", err)
	}
	if !strings.Contains(string(copilotSpecifyContent), "## Outline") {
		t.Fatalf("copilot speckit specify agent was not provisioned with real Spec Kit content")
	}
	promptContent, err := os.ReadFile(filepath.Join(".github", "prompts", "speckit.specify.prompt.md"))
	if err != nil {
		t.Fatalf("read speckit specify prompt: %v", err)
	}
	if !strings.Contains(string(promptContent), "agent: speckit.specify") {
		t.Fatalf("speckit specify prompt shim missing agent binding")
	}
	copilotInstructions, err := os.ReadFile(filepath.Join(".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("read copilot instructions: %v", err)
	}
	if !strings.Contains(string(copilotInstructions), "<!-- SPECKIT START -->") {
		t.Fatalf("copilot instructions missing Speckit managed section")
	}
	vscodeSettings, err := os.ReadFile(filepath.Join(".vscode", "settings.json"))
	if err != nil {
		t.Fatalf("read vscode settings: %v", err)
	}
	if !strings.Contains(string(vscodeSettings), "chat.promptFilesRecommendations") {
		t.Fatalf("vscode settings missing Copilot prompt recommendations")
	}
	scriptInfo, err := os.Stat(filepath.Join(".specify", "scripts", "bash", "setup-plan.sh"))
	if err != nil {
		t.Fatalf("stat setup-plan.sh: %v", err)
	}
	if scriptInfo.Mode()&0o111 == 0 {
		t.Fatalf("setup-plan.sh is not executable: mode=%#o", scriptInfo.Mode().Perm())
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

func TestSyncForceUpdatesUnmanagedProvisionedFiles(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("init returned %d\nstderr: %s", code, stderr.String())
	}
	path := filepath.Join(".dft", "flows", "spec-lane.yaml")
	if err := os.WriteFile(path, []byte("stale flow\n"), 0o644); err != nil {
		t.Fatalf("write stale flow: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"sync", "--force"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sync --force returned %d\nstderr: %s", code, stderr.String())
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read flow: %v", err)
	}
	if strings.Contains(string(content), "stale flow") {
		t.Fatalf("sync --force did not update unmanaged provisioned file: %q", content)
	}
	if !strings.Contains(string(content), "command: speckit.specify") {
		t.Fatalf("sync --force wrote unexpected flow: %q", content)
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
