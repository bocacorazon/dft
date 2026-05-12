package agentstub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

// Adapter deterministically converts supported agent requests into fixtures.
type Adapter struct{}

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
	case "dft-eval-plan-author.agent.md":
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
