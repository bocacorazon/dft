package domain

import "testing"

func TestEvalSurfaceContractValidateRequiresDeclaredSurface(t *testing.T) {
	contract := EvalSurfaceContract{
		DemandPackageID: "demand-1",
		Surfaces: []EvalSurface{{
			ID:               "cli",
			Kind:             EvalSurfaceCLI,
			ArtifactRef:      "cli-bin",
			AdapterFamily:    "cli",
			EnvironmentClass: EvalEnvironmentEphemeral,
			Readiness: []ReadinessProbe{{
				ID:   "binary-exists",
				Kind: ReadinessFileExists,
				Args: []string{"bin/app"},
			}},
		}},
	}

	if err := contract.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEvalPlanValidateRequiresExecutableContent(t *testing.T) {
	plan := EvalPlan{DemandPackageID: "demand-1"}

	if err := plan.Validate(); err == nil {
		t.Fatalf("Validate returned nil, want error")
	}
}

func TestEvalPlanValidateAcceptsBDDAndChecks(t *testing.T) {
	plan := EvalPlan{
		DemandPackageID: "demand-1",
		RequirementIDs:  []string{"REQ-001"},
		Packs: []BDDPack{{
			ID:        "cli-pack",
			SurfaceID: "cli",
			Scenarios: []BDDScenario{{
				ID:             "version",
				Name:           "prints version",
				RequirementIDs: []string{"REQ-001"},
				Steps: []BDDStep{{
					Phase:  "when",
					Action: "cli.run",
				}},
			}},
		}},
		Checks: []Check{{
			ID:   "no-binaries",
			Kind: CheckNoBinaryArtifacts,
		}},
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
