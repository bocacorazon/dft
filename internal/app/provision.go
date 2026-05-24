package app

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bocacorazon/dft/internal/orchestration"
)

const managedHeader = "---\nmanaged_by: dft\nversion: 1\n---\n"

type provisionedAsset struct {
	Path    string
	Content string
	Mode    os.FileMode
}

type provisionedHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

//go:embed provisioned/**
var provisionedSpecKitAssets embed.FS

func provisionAssets(command string, args []string, stdout io.Writer, stderr io.Writer) int {
	force := false
	for _, arg := range args {
		switch arg {
		case "--force":
			force = true
		default:
			fmt.Fprintf(stderr, "%s: unknown option %q\n", command, arg)
			return 2
		}
	}
	assets := defaultProvisionedAssets()
	hashes := make([]provisionedHash, 0, len(assets))
	previousHashes := map[string]string{}
	if command == "sync" {
		loaded, err := loadProvisioningManifest(filepath.Join(".dft", "provisioning-manifest.json"))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		previousHashes = loaded
	}
	for _, asset := range assets {
		if err := os.MkdirAll(filepath.Dir(asset.Path), 0o755); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		content := []byte(asset.Content)
		existing, err := os.ReadFile(asset.Path)
		if err == nil {
			if !force {
				switch command {
				case "init":
					if isManaged(existing) {
						hashes = append(hashes, hashAsset(asset.Path, existing))
					}
					continue
				case "sync":
					if !shouldUpdateProvisionedAsset(asset.Path, existing, previousHashes) {
						if previousHash, ok := previousHashes[filepath.ToSlash(filepath.Clean(asset.Path))]; ok {
							hashes = append(hashes, provisionedHash{Path: filepath.ToSlash(filepath.Clean(asset.Path)), SHA256: previousHash})
						}
						continue
					}
				}
			}
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(stderr, err)
			return 2
		}
		mode := asset.Mode
		if mode == 0 {
			mode = defaultAssetMode(asset.Path)
		}
		if err := os.WriteFile(asset.Path, content, mode); err != nil {
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
	dftAgents := []provisionedAsset{
		{Path: filepath.Join(".dft", "agents", "dft-intake.agent.md"), Content: dftIntakeAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-demand-package.agent.md"), Content: dftDemandPackageAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-wbs-builder.agent.md"), Content: dftWBSBuilderAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-lane-selector.agent.md"), Content: dftLaneSelectorAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-eval-surface-author.agent.md"), Content: dftEvalSurfaceAuthorAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-eval-plan-author.agent.md"), Content: dftEvalPlanAuthorAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-fix-planner.agent.md"), Content: dftFixPlannerAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-code-review.agent.md"), Content: dftCodeReviewAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-review.agent.md"), Content: dftReviewAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-mergeback.agent.md"), Content: dftMergebackAgent()},
	}
	githubAgents := make([]provisionedAsset, 0, len(dftAgents))
	for _, agent := range dftAgents {
		githubAgents = append(githubAgents, provisionedAsset{
			Path:    filepath.Join(".github", "agents", filepath.Base(agent.Path)),
			Content: agent.Content,
		})
	}
	assets := append([]provisionedAsset{}, dftAgents...)
	assets = append(assets, githubAgents...)
	assets = append(assets, embeddedSpecKitProvisionedAssets()...)
	assets = append(assets,
		provisionedAsset{Path: ".gitignore", Content: managedHeader + "\n# dft transient runtime artifacts\n.dft/runs/\n.dft/worktrees/\n.dft/state.db\n.dft/inbox/*\n!.dft/inbox/.gitkeep\n"},
		provisionedAsset{Path: filepath.Join(".github", "copilot", "agents", "dft-intake.agent.md"), Content: dftIntakeAgent()},
		provisionedAsset{Path: filepath.Join(".github", "copilot", "agents", "dft-code-review.agent.md"), Content: dftCodeReviewAgent()},
		provisionedAsset{Path: filepath.Join(".github", "copilot", "agents", "dft-review.agent.md"), Content: dftReviewAgent()},
		provisionedAsset{Path: filepath.Join(".github", "copilot", "agents", "dft-mergeback.agent.md"), Content: dftMergebackAgent()},
		provisionedAsset{Path: filepath.Join(".dft", "lanes", "spec.json"), Content: `{"name":"spec","flow":".dft/flows/spec-lane.yaml"}` + "\n"},
		provisionedAsset{Path: filepath.Join(".dft", "flows", "spec-lane.yaml"), Content: orchestration.SpecKitLaneFlowYAML()},
		provisionedAsset{Path: filepath.Join(".dft", "context", "constitution.md"), Content: managedHeader + "\n# dft context\n\nFollow repository constitution and mandatory TDD.\n"},
		provisionedAsset{Path: filepath.Join(".dft", "context", "project.md"), Content: managedHeader + "\n# Project context\n\nDescribe local project conventions, commands, and constraints here.\n"},
		provisionedAsset{Path: filepath.Join(".dft", "inbox", ".gitkeep"), Content: managedHeader + "\n"},
		provisionedAsset{Path: filepath.Join(".specify", "memory", "constitution.md"), Content: managedHeader + "\n# Constitution\n\nGo project with mandatory TDD and fix-all-tests policy.\n"},
	)
	return assets
}

func legacyDefaultProvisionedAssets() []provisionedAsset {
	return []provisionedAsset{
		{Path: ".gitignore", Content: managedHeader + "\n# dft transient runtime artifacts\n.dft/runs/\n.dft/worktrees/\n.dft/state.db\n.dft/inbox/*\n!.dft/inbox/.gitkeep\n"},
		{Path: filepath.Join(".dft", "agents", "dft-intake.agent.md"), Content: managedAgent("dft Intake Agent", "Normalize raw user demand into a demand package.", "Return strict JSON for a demand package.")},
		{Path: filepath.Join(".dft", "agents", "dft-demand-package.agent.md"), Content: managedAgent("dft Demand Package Agent", "Refine demand into bounded, testable work.", "Return strict JSON and keep scope v1-bounded.")},
		{Path: filepath.Join(".dft", "agents", "dft-wbs-builder.agent.md"), Content: managedAgent("dft WBS Builder Agent", "Decompose a demand package into independently executable specs.", "Return strict JSON WBS with acceptance criteria.")},
		{Path: filepath.Join(".dft", "agents", "dft-lane-selector.agent.md"), Content: managedAgent("dft Lane Selector Agent", "Assign each spec to an execution lane.", "Return strict JSON lane assignments.")},
		{Path: filepath.Join(".dft", "agents", "dft-eval-plan-author.agent.md"), Content: managedAgent("dft Eval Plan Author Agent", "Author adversarial deterministic verification checks.", "Return strict JSON evaluation plans.")},
		{Path: filepath.Join(".dft", "agents", "dft-fix-planner.agent.md"), Content: managedAgent("dft Fix Planner Agent", "Convert failed evaluation findings into WBS amendments.", "Return strict JSON remediation plans.")},
		{Path: filepath.Join(".dft", "agents", "dft-code-review.agent.md"), Content: managedAgent("dft Code Review Agent", "Perform code review during the Speckit implementation loop.", "Return strict JSON review decisions with severity.")},
		{Path: filepath.Join(".dft", "agents", "dft-review.agent.md"), Content: managedAgent("dft Review Agent", "Perform final code review before merge.", "Return strict JSON review decisions.")},
		{Path: filepath.Join(".dft", "agents", "dft-mergeback.agent.md"), Content: managedAgent("dft Mergeback Agent", "Resolve mergeback conflicts when a spec branch is rebased or merged into its target branch.", "Resolve git conflicts when present and otherwise do nothing.")},
		{Path: filepath.Join(".github", "copilot", "agents", "dft-intake.agent.md"), Content: managedAgent("dft Intake Agent", "Normalize raw user demand into a demand package.", "Return strict JSON for a demand package.")},
		{Path: filepath.Join(".github", "copilot", "agents", "dft-code-review.agent.md"), Content: managedAgent("dft Code Review Agent", "Perform code review during the Speckit implementation loop.", "Return strict JSON review decisions with severity.")},
		{Path: filepath.Join(".github", "copilot", "agents", "dft-review.agent.md"), Content: managedAgent("dft Review Agent", "Perform final code review before merge.", "Return strict JSON review decisions.")},
		{Path: filepath.Join(".github", "copilot", "agents", "dft-mergeback.agent.md"), Content: managedAgent("dft Mergeback Agent", "Resolve mergeback conflicts when a spec branch is rebased or merged into its target branch.", "Resolve git conflicts when present and otherwise do nothing.")},
		{Path: filepath.Join(".dft", "lanes", "spec.json"), Content: `{"name":"spec","flow":".dft/flows/spec-lane.yaml"}` + "\n"},
		{Path: filepath.Join(".dft", "flows", "spec-lane.yaml"), Content: orchestration.SpecKitLaneFlowYAML()},
		{Path: filepath.Join(".dft", "context", "constitution.md"), Content: managedHeader + "\n# dft context\n\nFollow repository constitution and mandatory TDD.\n"},
		{Path: filepath.Join(".dft", "context", "project.md"), Content: managedHeader + "\n# Project context\n\nDescribe local project conventions, commands, and constraints here.\n"},
		{Path: filepath.Join(".dft", "inbox", ".gitkeep"), Content: managedHeader + "\n"},
		{Path: filepath.Join(".specify", "memory", "constitution.md"), Content: managedHeader + "\n# Constitution\n\nGo project with mandatory TDD and fix-all-tests policy.\n"},
	}
}

func dftIntakeAgent() string {
	return managedAgent("dft Intake Agent", "Normalize raw user demand into a demand package.", `Return only JSON with this shape:
{
  "id": "run-id-from-prompt-when-known",
  "title": "short title",
  "raw_demand": "the original request",
  "acceptance_criteria": ["testable outcome"],
  "assumptions": ["reasonable assumption"],
  "non_goals": ["explicitly excluded work"]
}

Do not include markdown fences or commentary.`)
}

func dftDemandPackageAgent() string {
	return managedAgent("dft Demand Package Agent", "Refine demand into bounded, testable work.", `Return only JSON with the demand package shape:
{"id":"run-id","title":"short title","raw_demand":"original request","acceptance_criteria":["testable outcome"],"assumptions":[],"non_goals":[]}

Keep scope small enough for one increment. Do not include markdown fences or commentary.`)
}

func dftWBSBuilderAgent() string {
	return managedAgent("dft WBS Builder Agent", "Decompose a demand package into independently executable specs.", `Return only JSON with this shape:
{
  "demand_package_id": "id",
  "specs": [
    {"id": "001-short-name", "description": "one independently executable spec", "acceptance_criteria": ["testable criterion"]}
  ]
}

Prefer one spec for small single-artifact requests. Do not split one small CLI, API endpoint, file artifact, or documentation-only change into setup/help/test/docs specs unless the demand explicitly requires independent deliverables. Do not include markdown fences or commentary.`)
}

func dftLaneSelectorAgent() string {
	return managedAgent("dft Lane Selector Agent", "Assign each spec to an execution lane.", `Return only JSON with this shape:
[
  {"spec_id": "001-short-name", "lane": "spec", "rationale": "why this lane fits"}
]

Emit exactly one assignment per spec and prefer the "spec" lane. Do not include markdown fences or commentary.`)
}

func dftEvalSurfaceAuthorAgent() string {
	return managedAgent("dft Eval Surface Author Agent", "Declare artifact-only eval surfaces during solution design.", `Return only JSON with this shape:
{
  "demand_package_id": "id",
  "surfaces": [
    {
      "id": "cli",
      "kind": "cli",
      "artifact_ref": "cli-binary",
      "adapter_family": "cli",
      "environment_class": "ephemeral",
      "provisioning": "build_artifact",
      "readiness": [
        {"id":"binary-exists","kind":"file_exists","args":["bin/app"]}
      ],
      "reset_policy": "none",
      "evidence_policy": "capture_stdout_stderr"
    }
  ]
}

Author this during solution design from the demand package and WBS only. Do not inspect source code.
Default environment_class to "ephemeral"; use bound_external or live only when explicitly required.
Use kind values: cli, http_api, graphql, grpc, web_ui, file, event, database, infra, container, composite.
Use readiness kinds: file_exists, command_exit_zero, http_status.
Do not include markdown fences or commentary.`)
}

func dftEvalPlanAuthorAgent() string {
	return managedAgent("dft Eval Plan Author Agent", "Author adversarial artifact-only BDD evaluation plans.", `Return only JSON with this shape:
{
  "demand_package_id": "id",
  "requirement_ids": ["REQ-001"],
  "packs": [
    {
      "id": "cli-pack",
      "surface_id": "cli",
      "visibility": "hidden",
      "feature": "Feature: observable behavior",
      "scenarios": [
        {
          "id": "scenario-id",
          "name": "verifies one acceptance criterion",
          "requirement_ids": ["REQ-001"],
          "steps": [
            {"phase":"when","action":"cli.run","args":["--version"]},
            {"phase":"then","action":"cli.expect_exit_code","args":["0"]}
          ]
        }
      ]
    }
  ],
  "checks": []
}

Use only surfaces and artifacts provided in the prompt. Do not ask for implementation source and do not infer hidden surfaces from code.
Supported actions for the first slice: cli.run, cli.expect_exit_code, cli.expect_stdout_contains, cli.expect_stderr_contains, file.exists, file.contains, http.get, http.post_json, http.expect_status, http.expect_json_path_equals.
Optional deterministic checks may use these kinds: file_exists, file_missing, command_exit_zero, grep_matches, json_path_equals, count_matches_at_least, count_matches_equals, file_checksum_differs, git_no_unmerged_files, string_equals, os, no_binary_artifacts.
Use hidden visibility unless explicitly told to publish. Do not include markdown fences or commentary.`)
}

func dftFixPlannerAgent() string {
	return managedAgent("dft Fix Planner Agent", "Convert failed evaluation findings into WBS amendments.", `Return only JSON with this shape:
{
  "demand_package_id": "id",
  "findings": [{"check_id":"failed-check","message":"what failed"}],
  "remediation_specs": [
    {"id":"fix-short-name","description":"remediation spec","acceptance_criteria":["testable criterion"]}
  ]
}

Prefer remediation specs for defects inside the current increment. Do not include markdown fences or commentary.`)
}

func dftReviewAgent() string {
	return managedAgent("dft Review Agent", "Perform final code review before merge.", `Return only JSON with this shape:
{"approved": true, "findings": [{"severity":"HIGH","message":"clear actionable finding"}]}

Use severity values CRITICAL, HIGH, MEDIUM, or LOW.
Block only correctness, security, data-loss, auditability, or test coverage issues. If not approved, include at least one finding with severity and message. Do not include markdown fences or commentary.`)
}

func dftCodeReviewAgent() string {
	return managedAgent("dft Code Review Agent", "Perform code review during the Speckit implementation loop.", `Return only JSON with this shape:
{"approved": true, "findings": [{"severity":"HIGH","message":"clear actionable finding"}]}

Use severity values CRITICAL, HIGH, MEDIUM, or LOW.
Block only correctness, security, data-loss, auditability, or test coverage issues. If not approved, include at least one finding with severity and message. Do not include markdown fences or commentary.`)
}

func dftMergebackAgent() string {
	return managedAgent("dft Mergeback Agent", "Resolve mergeback conflicts when a spec branch is being rebased onto its target branch.", `If a rebase conflict is present in the current repository, resolve it carefully, finish the rebase, and leave the repository clean and ready for the engine to perform the squash merge and branch deletion steps.

If there is no active conflict and the rebase is already complete, do nothing.
Do not create new feature work; only resolve the in-progress rebase operation.`)
}

func speckitAgent(title string, body string) string {
	return managedAgent(title, body, body+"\n\nDo not commit changes. dft owns commits. Prefer deterministic tests and local-only dependencies.")
}

func embeddedSpecKitProvisionedAssets() []provisionedAsset {
	var assets []provisionedAsset
	_ = fs.WalkDir(provisionedSpecKitAssets, "provisioned", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := provisionedSpecKitAssets.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		switch {
		case strings.HasPrefix(path, "provisioned/github/agents/"):
			commandName := strings.TrimSuffix(filepath.Base(path), ".agent.md")
			assets = append(assets, provisionedAsset{
				Path:    filepath.Join(".github", "agents", filepath.Base(path)),
				Content: string(content),
			})
			assets = append(assets, provisionedAsset{
				Path:    filepath.Join(".github", "copilot", "agents", filepath.Base(path)),
				Content: string(content),
			})
			assets = append(assets, provisionedAsset{
				Path:    filepath.Join(".github", "prompts", commandName+".prompt.md"),
				Content: "---\nagent: " + commandName + "\n---\n",
			})
		case path == "provisioned/github/copilot-instructions.md":
			assets = append(assets, provisionedAsset{
				Path:    filepath.Join(".github", "copilot-instructions.md"),
				Content: string(content),
			})
		case path == "provisioned/vscode/settings.json":
			assets = append(assets, provisionedAsset{
				Path:    filepath.Join(".vscode", "settings.json"),
				Content: string(content),
			})
		case strings.HasPrefix(path, "provisioned/specify/"):
			relative := strings.TrimPrefix(path, "provisioned/specify/")
			assets = append(assets, provisionedAsset{
				Path:    filepath.Join(".specify", filepath.FromSlash(relative)),
				Content: string(content),
				Mode:    defaultAssetMode(relative),
			})
		}
		return nil
	})
	return assets
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

func shouldUpdateProvisionedAsset(path string, existing []byte, previousHashes map[string]string) bool {
	if isManaged(existing) {
		return true
	}
	previousHash, ok := previousHashes[filepath.ToSlash(filepath.Clean(path))]
	if !ok {
		return false
	}
	return hashAsset(path, existing).SHA256 == previousHash
}

func defaultAssetMode(path string) os.FileMode {
	if strings.HasSuffix(path, ".sh") {
		return 0o755
	}
	return 0o644
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

func loadProvisioningManifest(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read provisioning manifest: %w", err)
	}
	var manifest struct {
		Assets []provisionedHash `json:"assets"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("parse provisioning manifest: %w", err)
	}
	hashes := make(map[string]string, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		hashes[asset.Path] = asset.SHA256
	}
	return hashes, nil
}
