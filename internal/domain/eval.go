package domain

import "fmt"

// EvalStatus is the aggregate artifact-evaluation outcome.
type EvalStatus string

const (
	EvalStatusPass    EvalStatus = "pass"
	EvalStatusFail    EvalStatus = "fail"
	EvalStatusError   EvalStatus = "error"
	EvalStatusBlocked EvalStatus = "blocked"
)

// EvalSurfaceKind names an externally observable artifact/test surface.
type EvalSurfaceKind string

const (
	EvalSurfaceCLI       EvalSurfaceKind = "cli"
	EvalSurfaceHTTPAPI   EvalSurfaceKind = "http_api"
	EvalSurfaceGraphQL   EvalSurfaceKind = "graphql"
	EvalSurfaceGRPC      EvalSurfaceKind = "grpc"
	EvalSurfaceWebUI     EvalSurfaceKind = "web_ui"
	EvalSurfaceFile      EvalSurfaceKind = "file"
	EvalSurfaceEvent     EvalSurfaceKind = "event"
	EvalSurfaceDatabase  EvalSurfaceKind = "database"
	EvalSurfaceInfra     EvalSurfaceKind = "infra"
	EvalSurfaceContainer EvalSurfaceKind = "container"
	EvalSurfaceComposite EvalSurfaceKind = "composite"
)

// EvalEnvironmentClass controls where artifact-only evaluation may run.
type EvalEnvironmentClass string

const (
	EvalEnvironmentEphemeral     EvalEnvironmentClass = "ephemeral"
	EvalEnvironmentBoundExternal EvalEnvironmentClass = "bound_external"
	EvalEnvironmentLive          EvalEnvironmentClass = "live"
)

// ReadinessProbeKind is a closed-set readiness predicate for eval targets.
type ReadinessProbeKind string

const (
	ReadinessFileExists      ReadinessProbeKind = "file_exists"
	ReadinessCommandExitZero ReadinessProbeKind = "command_exit_zero"
	ReadinessHTTPStatus      ReadinessProbeKind = "http_status"
)

// ArtifactKind names a delivered artifact type.
type ArtifactKind string

const (
	ArtifactBinary    ArtifactKind = "binary"
	ArtifactFile      ArtifactKind = "file"
	ArtifactDirectory ArtifactKind = "directory"
	ArtifactImage     ArtifactKind = "image"
	ArtifactURL       ArtifactKind = "url"
	ArtifactSchema    ArtifactKind = "schema"
	ArtifactTopic     ArtifactKind = "topic"
)

// EvalSurface declares one public surface that can prove requirements.
type EvalSurface struct {
	ID               string               `json:"id"`
	Kind             EvalSurfaceKind      `json:"kind"`
	ArtifactRef      string               `json:"artifact_ref"`
	AdapterFamily    string               `json:"adapter_family"`
	EnvironmentClass EvalEnvironmentClass `json:"environment_class"`
	Provisioning     string               `json:"provisioning,omitempty"`
	Readiness        []ReadinessProbe     `json:"readiness,omitempty"`
	ResetPolicy      string               `json:"reset_policy,omitempty"`
	EvidencePolicy   string               `json:"evidence_policy,omitempty"`
}

// ReadinessProbe proves that a bound eval target is ready.
type ReadinessProbe struct {
	ID   string             `json:"id"`
	Kind ReadinessProbeKind `json:"kind"`
	Args []string           `json:"args,omitempty"`
}

// EvalSurfaceContract is authored with the WBS during solution design.
type EvalSurfaceContract struct {
	DemandPackageID string        `json:"demand_package_id"`
	Surfaces        []EvalSurface `json:"surfaces"`
}

// ArtifactRef identifies a delivered artifact or endpoint.
type ArtifactRef struct {
	ID   string       `json:"id"`
	Kind ArtifactKind `json:"kind"`
	URI  string       `json:"uri,omitempty"`
	Path string       `json:"path,omitempty"`
}

// ArtifactManifest lists artifacts collected after WBS completion.
type ArtifactManifest struct {
	DemandPackageID string        `json:"demand_package_id"`
	Artifacts       []ArtifactRef `json:"artifacts"`
}

// SurfaceBinding connects one declared surface to one delivered artifact.
type SurfaceBinding struct {
	SurfaceID  string       `json:"surface_id"`
	ArtifactID string       `json:"artifact_id"`
	Kind       ArtifactKind `json:"kind"`
	URI        string       `json:"uri,omitempty"`
	Path       string       `json:"path,omitempty"`
}

// EvalReady proves all required eval targets are available and ready.
type EvalReady struct {
	Status   EvalStatus       `json:"status"`
	Bindings []SurfaceBinding `json:"bindings,omitempty"`
	Findings []Finding        `json:"findings,omitempty"`
}

// EvalVisibility controls whether a pack is exposed to implementation agents.
type EvalVisibility string

const (
	EvalVisibilityHidden    EvalVisibility = "hidden"
	EvalVisibilityPublished EvalVisibility = "published"
)

// BDDStep is a generic, executor-neutral scenario step.
type BDDStep struct {
	Phase  string   `json:"phase"`
	Action string   `json:"action"`
	Args   []string `json:"args,omitempty"`
}

// BDDScenario is one requirement-tagged behavior check.
type BDDScenario struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	RequirementIDs []string  `json:"requirement_ids,omitempty"`
	Steps          []BDDStep `json:"steps"`
}

// BDDPack groups scenarios bound to one surface.
type BDDPack struct {
	ID         string         `json:"id"`
	SurfaceID  string         `json:"surface_id"`
	Visibility EvalVisibility `json:"visibility,omitempty"`
	Feature    string         `json:"feature,omitempty"`
	Scenarios  []BDDScenario  `json:"scenarios"`
}

// EvalPlan is the artifact-only BDD plan consumed by the eval executor.
type EvalPlan struct {
	DemandPackageID string    `json:"demand_package_id"`
	RequirementIDs  []string  `json:"requirement_ids,omitempty"`
	Packs           []BDDPack `json:"packs,omitempty"`
	Checks          []Check   `json:"checks,omitempty"`
}

// PackExecution records one pack execution result.
type PackExecution struct {
	PackID    string            `json:"pack_id"`
	SurfaceID string            `json:"surface_id"`
	Status    EvalStatus        `json:"status"`
	Scenarios []ScenarioOutcome `json:"scenarios,omitempty"`
	Findings  []Finding         `json:"findings,omitempty"`
}

// ScenarioOutcome records one scenario execution result.
type ScenarioOutcome struct {
	ScenarioID     string     `json:"scenario_id"`
	RequirementIDs []string   `json:"requirement_ids,omitempty"`
	Status         EvalStatus `json:"status"`
	Message        string     `json:"message,omitempty"`
	Evidence       []Evidence `json:"evidence,omitempty"`
}

// Evidence references an artifact captured during evaluation.
type Evidence struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	URI  string `json:"uri,omitempty"`
	Path string `json:"path,omitempty"`
}

// RequirementCoverage summarizes pass coverage for declared requirements.
type RequirementCoverage struct {
	Total     int      `json:"total"`
	Covered   int      `json:"covered"`
	Uncovered []string `json:"uncovered,omitempty"`
}

// EvalResult is the aggregate artifact-only evaluation verdict.
type EvalResult struct {
	Status     EvalStatus          `json:"status"`
	Readiness  *EvalReady          `json:"readiness,omitempty"`
	Executions []PackExecution     `json:"executions,omitempty"`
	Coverage   RequirementCoverage `json:"coverage"`
	Findings   []Finding           `json:"findings,omitempty"`
	Checks     []CheckResult       `json:"checks,omitempty"`
}

// Validate returns an error when the surface contract cannot guide eval.
func (c EvalSurfaceContract) Validate() error {
	if c.DemandPackageID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if len(c.Surfaces) == 0 {
		return fmt.Errorf("at least one eval surface is required")
	}
	seen := map[string]struct{}{}
	for _, surface := range c.Surfaces {
		if surface.ID == "" {
			return fmt.Errorf("eval surface id is required")
		}
		if _, ok := seen[surface.ID]; ok {
			return fmt.Errorf("duplicate eval surface %q", surface.ID)
		}
		seen[surface.ID] = struct{}{}
		if surface.Kind == "" {
			return fmt.Errorf("eval surface %q kind is required", surface.ID)
		}
		if surface.ArtifactRef == "" {
			return fmt.Errorf("eval surface %q artifact_ref is required", surface.ID)
		}
		if surface.AdapterFamily == "" {
			return fmt.Errorf("eval surface %q adapter_family is required", surface.ID)
		}
		if surface.EnvironmentClass == "" {
			return fmt.Errorf("eval surface %q environment_class is required", surface.ID)
		}
		for _, probe := range surface.Readiness {
			if probe.ID == "" {
				return fmt.Errorf("eval surface %q readiness probe id is required", surface.ID)
			}
			if probe.Kind == "" {
				return fmt.Errorf("eval surface %q readiness probe %q kind is required", surface.ID, probe.ID)
			}
		}
	}
	return nil
}

// Validate returns an error when the manifest cannot bind surfaces.
func (m ArtifactManifest) Validate() error {
	if m.DemandPackageID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("at least one artifact is required")
	}
	seen := map[string]struct{}{}
	for _, artifact := range m.Artifacts {
		if artifact.ID == "" {
			return fmt.Errorf("artifact id is required")
		}
		if _, ok := seen[artifact.ID]; ok {
			return fmt.Errorf("duplicate artifact %q", artifact.ID)
		}
		seen[artifact.ID] = struct{}{}
		if artifact.Kind == "" {
			return fmt.Errorf("artifact %q kind is required", artifact.ID)
		}
		if artifact.URI == "" && artifact.Path == "" {
			return fmt.Errorf("artifact %q uri or path is required", artifact.ID)
		}
	}
	return nil
}

// Validate returns an error when the eval plan is not executable.
func (p EvalPlan) Validate() error {
	if p.DemandPackageID == "" {
		return fmt.Errorf("demand package id is required")
	}
	if len(p.Packs) == 0 && len(p.Checks) == 0 {
		return fmt.Errorf("at least one BDD pack or deterministic check is required")
	}
	seenPacks := map[string]struct{}{}
	for _, pack := range p.Packs {
		if pack.ID == "" {
			return fmt.Errorf("BDD pack id is required")
		}
		if _, ok := seenPacks[pack.ID]; ok {
			return fmt.Errorf("duplicate BDD pack %q", pack.ID)
		}
		seenPacks[pack.ID] = struct{}{}
		if pack.SurfaceID == "" {
			return fmt.Errorf("BDD pack %q surface_id is required", pack.ID)
		}
		if len(pack.Scenarios) == 0 {
			return fmt.Errorf("BDD pack %q requires at least one scenario", pack.ID)
		}
		for _, scenario := range pack.Scenarios {
			if scenario.ID == "" {
				return fmt.Errorf("BDD pack %q scenario id is required", pack.ID)
			}
			if scenario.Name == "" {
				return fmt.Errorf("BDD pack %q scenario %q name is required", pack.ID, scenario.ID)
			}
			if len(scenario.Steps) == 0 {
				return fmt.Errorf("BDD pack %q scenario %q requires steps", pack.ID, scenario.ID)
			}
			for _, step := range scenario.Steps {
				if step.Action == "" {
					return fmt.Errorf("BDD pack %q scenario %q step action is required", pack.ID, scenario.ID)
				}
			}
		}
	}
	for _, check := range p.Checks {
		if check.ID == "" {
			return fmt.Errorf("evaluation check id is required")
		}
		if check.Kind == "" {
			return fmt.Errorf("evaluation check %q kind is required", check.ID)
		}
	}
	return nil
}
