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
	dftAgents := []provisionedAsset{
		{Path: filepath.Join(".dft", "agents", "dft-intake.agent.md"), Content: dftIntakeAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-demand-package.agent.md"), Content: dftDemandPackageAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-wbs-builder.agent.md"), Content: dftWBSBuilderAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-lane-selector.agent.md"), Content: dftLaneSelectorAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-eval-plan-author.agent.md"), Content: dftEvalPlanAuthorAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-fix-planner.agent.md"), Content: dftFixPlannerAgent()},
		{Path: filepath.Join(".dft", "agents", "dft-review.agent.md"), Content: dftReviewAgent()},
	}
	githubAgents := make([]provisionedAsset, 0, len(dftAgents)+4)
	for _, agent := range dftAgents {
		githubAgents = append(githubAgents, provisionedAsset{
			Path:    filepath.Join(".github", "agents", filepath.Base(agent.Path)),
			Content: agent.Content,
		})
	}
	githubAgents = append(githubAgents,
		provisionedAsset{Path: filepath.Join(".github", "agents", "speckit.specify.agent.md"), Content: speckitAgent("Spec Kit Specify Agent", "Create or update the feature specification from the provided feature description. Write the specification under the feature directory when possible.")},
		provisionedAsset{Path: filepath.Join(".github", "agents", "speckit.plan.agent.md"), Content: speckitAgent("Spec Kit Plan Agent", "Create the technical implementation plan for the current feature spec. Use the repository constitution and existing project context.")},
		provisionedAsset{Path: filepath.Join(".github", "agents", "speckit.tasks.agent.md"), Content: speckitAgent("Spec Kit Tasks Agent", "Generate actionable implementation tasks for the current feature plan. Prefer TDD and deterministic verification.")},
		provisionedAsset{Path: filepath.Join(".github", "agents", "speckit.implement.agent.md"), Content: speckitAgent("Spec Kit Implement Agent", "Implement the current feature tasks in the repository. Write tests first where practical, run project validation, and leave changes in the worktree for dft to commit.")},
	)
	assets := append([]provisionedAsset{}, dftAgents...)
	assets = append(assets, githubAgents...)
	assets = append(assets,
		provisionedAsset{Path: filepath.Join(".github", "copilot", "agents", "dft-intake.agent.md"), Content: dftIntakeAgent()},
		provisionedAsset{Path: filepath.Join(".github", "copilot", "agents", "dft-review.agent.md"), Content: dftReviewAgent()},
		provisionedAsset{Path: filepath.Join(".dft", "lanes", "spec.json"), Content: `{"name":"spec","flow":".dft/flows/spec-lane.json"}` + "\n"},
		provisionedAsset{Path: filepath.Join(".dft", "flows", "spec-lane.json"), Content: `{"max_spec_parallelism":1,"steps":[{"id":"implement","type":"agent","agent_name":"dft-intake.agent.md","prompt":"Execute the spec lane","max_iterations":1}]}` + "\n"},
		provisionedAsset{Path: filepath.Join(".dft", "context", "constitution.md"), Content: managedHeader + "\n# dft context\n\nFollow repository constitution and mandatory TDD.\n"},
		provisionedAsset{Path: filepath.Join(".dft", "context", "project.md"), Content: managedHeader + "\n# Project context\n\nDescribe local project conventions, commands, and constraints here.\n"},
		provisionedAsset{Path: filepath.Join(".dft", "inbox", ".gitkeep"), Content: managedHeader + "\n"},
		provisionedAsset{Path: filepath.Join(".specify", "memory", "constitution.md"), Content: managedHeader + "\n# Constitution\n\nGo project with mandatory TDD and fix-all-tests policy.\n"},
	)
	return assets
}

func legacyDefaultProvisionedAssets() []provisionedAsset {
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

Prefer a small sequential WBS. Do not include markdown fences or commentary.`)
}

func dftLaneSelectorAgent() string {
	return managedAgent("dft Lane Selector Agent", "Assign each spec to an execution lane.", `Return only JSON with this shape:
[
  {"spec_id": "001-short-name", "lane": "spec", "rationale": "why this lane fits"}
]

Emit exactly one assignment per spec and prefer the "spec" lane. Do not include markdown fences or commentary.`)
}

func dftEvalPlanAuthorAgent() string {
	return managedAgent("dft Eval Plan Author Agent", "Author adversarial deterministic verification checks.", `Return only JSON with this shape:
{"checks":[{"id":"check-id","kind":"command_exit_zero","args":["go","test","./..."]}]}

Allowed kinds: file_exists, file_missing, command_exit_zero, grep_matches, json_path_equals, count_matches_at_least, os, no_binary_artifacts.
Use argv arrays for commands and no shell interpretation. For Go apps, include go test ./... when applicable. Do not include markdown fences or commentary.`)
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
{"approved": true, "findings": []}

Block only correctness, security, data-loss, auditability, or test coverage issues. If not approved, include at least one finding with a message. Do not include markdown fences or commentary.`)
}

func speckitAgent(title string, body string) string {
	return managedAgent(title, body, body+"\n\nDo not commit changes. dft owns commits. Prefer deterministic tests and local-only dependencies.")
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
