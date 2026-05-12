package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const managedHeader = "---\nmanaged_by: dft\nversion: 1\n---\n"

type provisionedAsset struct {
	Path    string
	Content string
}

type provisionedHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func provisionAssets(command string, stdout io.Writer, stderr io.Writer) int {
	assets := defaultProvisionedAssets()
	hashes := make([]provisionedHash, 0, len(assets))
	for _, asset := range assets {
		if err := os.MkdirAll(filepath.Dir(asset.Path), 0o755); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		content := []byte(asset.Content)
		if command == "init" || command == "sync" {
			existing, err := os.ReadFile(asset.Path)
			if err == nil {
				if command == "init" || !isManaged(existing) {
					hashes = append(hashes, hashAsset(asset.Path, existing))
					continue
				}
			} else if !os.IsNotExist(err) {
				fmt.Fprintln(stderr, err)
				return 2
			}
		}
		if err := os.WriteFile(asset.Path, content, 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		hashes = append(hashes, hashAsset(asset.Path, content))
	}
	if err := writeProvisioningManifest(hashes); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "dft %s complete\n", command)
	return 0
}

func defaultProvisionedAssets() []provisionedAsset {
	return []provisionedAsset{
		{Path: filepath.Join(".dft", "agents", "dft-intake.agent.md"), Content: managedAgent("dft Intake Agent", "Normalize raw user demand into a demand package.", "Return strict JSON for a demand package.")},
		{Path: filepath.Join(".dft", "agents", "dft-demand-package.agent.md"), Content: managedAgent("dft Demand Package Agent", "Refine demand into bounded, testable work.", "Return strict JSON and keep scope v1-bounded.")},
		{Path: filepath.Join(".dft", "agents", "dft-wbs-builder.agent.md"), Content: managedAgent("dft WBS Builder Agent", "Decompose a demand package into independently executable specs.", "Return strict JSON WBS with acceptance criteria.")},
		{Path: filepath.Join(".dft", "agents", "dft-lane-selector.agent.md"), Content: managedAgent("dft Lane Selector Agent", "Assign each spec to an execution lane.", "Return strict JSON lane assignments.")},
		{Path: filepath.Join(".dft", "agents", "dft-eval-plan-author.agent.md"), Content: managedAgent("dft Eval Plan Author Agent", "Author adversarial deterministic verification checks.", "Return strict JSON evaluation plans.")},
		{Path: filepath.Join(".dft", "agents", "dft-fix-planner.agent.md"), Content: managedAgent("dft Fix Planner Agent", "Convert failed evaluation findings into WBS amendments.", "Return strict JSON remediation plans.")},
		{Path: filepath.Join(".dft", "agents", "dft-review.agent.md"), Content: managedAgent("dft Review Agent", "Perform final code review before merge.", "Return strict JSON review decisions.")},
		{Path: filepath.Join(".github", "copilot", "agents", "dft-intake.agent.md"), Content: managedAgent("dft Intake Agent", "Normalize raw user demand into a demand package.", "Return strict JSON for a demand package.")},
		{Path: filepath.Join(".github", "copilot", "agents", "dft-review.agent.md"), Content: managedAgent("dft Review Agent", "Perform final code review before merge.", "Return strict JSON review decisions.")},
		{Path: filepath.Join(".dft", "lanes", "spec.json"), Content: `{"name":"spec","flow":".dft/flows/spec-lane.json"}` + "\n"},
		{Path: filepath.Join(".dft", "flows", "spec-lane.json"), Content: `{"max_spec_parallelism":1,"steps":[{"id":"implement","type":"agent","agent_name":"dft-intake.agent.md","prompt":"Execute the spec lane","max_iterations":1}]}` + "\n"},
		{Path: filepath.Join(".dft", "context", "constitution.md"), Content: managedHeader + "\n# dft context\n\nFollow repository constitution and mandatory TDD.\n"},
		{Path: filepath.Join(".dft", "context", "project.md"), Content: managedHeader + "\n# Project context\n\nDescribe local project conventions, commands, and constraints here.\n"},
		{Path: filepath.Join(".dft", "inbox", ".gitkeep"), Content: managedHeader + "\n"},
		{Path: filepath.Join(".specify", "memory", "constitution.md"), Content: managedHeader + "\n# Constitution\n\nGo project with mandatory TDD and fix-all-tests policy.\n"},
	}
}

func managedAgent(title string, description string, body string) string {
	return "---\nmanaged_by: dft\nversion: 1\ndescription: " + description + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

func isManaged(content []byte) bool {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return false
	}
	end := strings.Index(text[4:], "---")
	if end < 0 {
		return false
	}
	header := text[:end+4]
	return strings.Contains(header, "managed_by: dft") && strings.Contains(header, "version:")
}

func hashAsset(path string, content []byte) provisionedHash {
	sum := sha256.Sum256(content)
	return provisionedHash{Path: filepath.ToSlash(filepath.Clean(path)), SHA256: hex.EncodeToString(sum[:])}
}

func writeProvisioningManifest(hashes []provisionedHash) error {
	path := filepath.Join(".dft", "provisioning-manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provisioning manifest directory: %w", err)
	}
	content, err := json.MarshalIndent(struct {
		Assets []provisionedHash `json:"assets"`
	}{Assets: hashes}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode provisioning manifest: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write provisioning manifest: %w", err)
	}
	return nil
}
