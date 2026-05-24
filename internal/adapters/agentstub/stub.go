package agentstub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// Adapter deterministically converts supported agent requests into fixtures.
type Adapter struct{}

// DispatchCommand returns deterministic text for a named workflow command.
func (Adapter) DispatchCommand(ctx context.Context, request ports.CommandRequest) (ports.CommandResponse, error) {
	if err := writeSpecKitArtifacts(request); err != nil {
		return ports.CommandResponse{}, err
	}
	return ports.CommandResponse{
		Stdout:   "stub dispatched " + request.Command + "\n",
		ExitCode: 0,
	}, nil
}

// Invoke returns strict JSON for the requested stub agent.
func (Adapter) Invoke(_ context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	switch request.AgentName {
	case "dft-wbs-builder.agent.md":
		return marshal(domain.WBS{
			DemandPackageID: request.RunID,
			Specs: []domain.SpecRef{{
				ID:          "001-" + slug(request.Demand),
				Description: request.Demand,
				AcceptanceCriteria: []string{
					"Spec can be executed independently by the selected lane.",
				},
			}},
		})
	case "dft-lane-selector.agent.md":
		return marshal([]domain.LaneAssignment{{
			SpecID:    "001-" + slug(request.Demand),
			Lane:      "spec",
			Rationale: "Stub bootstrap uses the full spec lane for deterministic coverage.",
		}})
	case "dft-eval-surface-author.agent.md":
		designWBSPath := ".dft/runs/" + request.RunID + "/design/wbs.json"
		return marshal(domain.EvalSurfaceContract{
			DemandPackageID: request.RunID,
			Surfaces: []domain.EvalSurface{{
				ID:               "stub-design-artifacts",
				Kind:             domain.EvalSurfaceFile,
				ArtifactRef:      designWBSPath,
				AdapterFamily:    "file",
				EnvironmentClass: domain.EvalEnvironmentEphemeral,
				Readiness: []domain.ReadinessProbe{{
					ID:   "wbs-artifact-exists",
					Kind: domain.ReadinessFileExists,
					Args: []string{designWBSPath},
				}},
			}},
		})
	case "dft-eval-plan-author.agent.md":
		if strings.Contains(request.Prompt, "artifact-only BDD eval plan") {
			return marshal(domain.EvalPlan{
				DemandPackageID: request.RunID,
				RequirementIDs:  []string{"REQ-STUB"},
				Packs: []domain.BDDPack{{
					ID:         "stub-file-pack",
					SurfaceID:  "stub-design-artifacts",
					Visibility: domain.EvalVisibilityHidden,
					Scenarios: []domain.BDDScenario{{
						ID:             "stub-file-exists",
						Name:           "stub eval artifact exists",
						RequirementIDs: []string{"REQ-STUB"},
						Steps: []domain.BDDStep{{
							Phase:  "then",
							Action: "file.exists",
							Args:   []string{".dft/runs/" + request.RunID + "/design/wbs.json"},
						}},
					}},
				}},
				Checks: []domain.Check{{
					ID:   "stub-deterministic-eval",
					Kind: domain.CheckFileExists,
					Args: []string{".dft/runs/" + request.RunID + "/design/wbs.json"},
				}},
			})
		}
		return marshal(domain.EvaluationPlan{Checks: []domain.Check{
			{ID: "wbs", Kind: domain.CheckFileExists, Args: []string{".dft/runs/" + request.RunID + "/design/wbs.json"}},
			{ID: "lane-assignments", Kind: domain.CheckFileExists, Args: []string{".dft/runs/" + request.RunID + "/design/lane-assignments.json"}},
		}})
	case "dft-fix-planner.agent.md":
		return marshal(domain.WBSAmendment{
			DemandPackageID: request.RunID,
			Findings: []domain.Finding{{
				CheckID: "stub-finding",
				Message: "Stub fix planner mirrors the failed evaluation into a remediation spec.",
			}},
			RemediationSpecs: []domain.SpecRef{{
				ID:          "fix-" + slug(request.Demand),
				Description: "Remediate failed evaluation findings for " + summarize(request.Demand),
				AcceptanceCriteria: []string{
					"Failed evaluation findings are corrected and the eval plan passes.",
				},
			}},
		})
	case "dft-review.agent.md":
		return marshal(domain.ReviewDecision{
			Approved: true,
		})
	case "dft-code-review.agent.md":
		return marshal(domain.ReviewDecision{
			Approved: true,
		})
	}

	title := summarize(request.Demand)
	return marshal(domain.DemandPackage{
		ID:        request.RunID,
		Title:     title,
		RawDemand: request.Demand,
		AcceptanceCriteria: []string{
			"Generated demand package preserves the original request.",
			"Generated demand package is specific enough for WBS decomposition.",
		},
		Assumptions: []string{
			"Stub adapter is being used for deterministic bootstrap execution.",
		},
	})
}

func marshal(value any) (ports.AgentResponse, error) {
	output, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ports.AgentResponse{}, fmt.Errorf("marshal stub output: %w", err)
	}
	return ports.AgentResponse{Raw: string(output) + "\n"}, nil
}

func summarize(demand string) string {
	words := strings.Fields(demand)
	if len(words) == 0 {
		return "Untitled demand"
	}
	if len(words) > 6 {
		words = words[:6]
	}
	return strings.Join(words, " ")
}

func slug(demand string) string {
	words := strings.Fields(strings.ToLower(demand))
	if len(words) == 0 {
		return "untitled"
	}
	if len(words) > 4 {
		words = words[:4]
	}
	for i, word := range words {
		words[i] = strings.Trim(word, "-_.,:;!?")
	}
	return strings.Join(words, "-")
}

func writeSpecKitArtifacts(request ports.CommandRequest) error {
	featureDir, ok := specKitFeatureDir(request)
	if !ok {
		return nil
	}
	if err := ensureSpecKitTemplates(request.Cwd); err != nil {
		return err
	}
	switch request.Command {
	case "speckit.specify":
		if err := os.MkdirAll(filepath.Join(featureDir, "checklists"), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(request.Cwd, ".specify"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(featureDir, "spec.md"), []byte("# Spec\n"), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(featureDir, "checklists", "requirements.md"), []byte("# Requirements\n"), 0o644); err != nil {
			return err
		}
		content, err := json.MarshalIndent(map[string]string{
			"feature_directory": filepath.ToSlash(request.Env["SPECIFY_FEATURE_DIRECTORY"]),
		}, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(request.Cwd, ".specify", "feature.json"), append(content, '\n'), 0o644)
	case "speckit.plan":
		if err := os.MkdirAll(filepath.Join(featureDir, "contracts"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(featureDir, "plan.md"), []byte("# Plan\n"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(featureDir, "research.md"), []byte("# Research\n"), 0o644)
	case "speckit.tasks":
		if err := os.MkdirAll(featureDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(featureDir, "tasks.md"), []byte("- [ ] Implement feature\n"), 0o644)
	case "speckit.implement":
		tasksPath := filepath.Join(featureDir, "tasks.md")
		content, err := os.ReadFile(tasksPath)
		if err != nil {
			return err
		}
		updated := strings.ReplaceAll(string(content), "[ ]", "[X]")
		if updated == string(content) {
			updated += "- [X] Implement feature\n"
		}
		return os.WriteFile(tasksPath, []byte(updated), 0o644)
	default:
		return nil
	}
}

func ensureSpecKitTemplates(root string) error {
	if root == "" {
		root = "."
	}
	templates := map[string]string{
		filepath.Join(".specify", "templates", "spec-template.md"):  "# Spec Template\n",
		filepath.Join(".specify", "templates", "plan-template.md"):  "# Plan Template\n",
		filepath.Join(".specify", "templates", "tasks-template.md"): "# Tasks Template\n",
	}
	for relative, content := range templates {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func specKitFeatureDir(request ports.CommandRequest) (string, bool) {
	relative := strings.TrimSpace(request.Env["SPECIFY_FEATURE_DIRECTORY"])
	if relative == "" {
		return "", false
	}
	if filepath.IsAbs(relative) {
		return relative, true
	}
	base := request.Cwd
	if base == "" {
		base = "."
	}
	return filepath.Join(base, relative), true
}
