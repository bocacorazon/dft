package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bocacorazon/dft/internal/adapters/state"
	"github.com/bocacorazon/dft/internal/domain"
	domainv2 "github.com/bocacorazon/dft/internal/domain/v2"
	orchv2 "github.com/bocacorazon/dft/internal/orchestration/v2"
)

func runBuild(args []string, stdout io.Writer, stderr io.Writer) int {
	adapterName := "stub"
	copilotBinary := ""
	agentTimeout := 30 * time.Minute
	singleSpec := ""
	resumeMode := false

	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--adapter":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--adapter requires a value")
				return 2
			}
			i++
			adapterName = args[i]
		case "--copilot-binary":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--copilot-binary requires a value")
				return 2
			}
			i++
			copilotBinary = args[i]
		case "--agent-timeout":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--agent-timeout requires a duration")
				return 2
			}
			i++
			parsed, err := time.ParseDuration(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, "--agent-timeout requires a positive duration, for example 30m")
				return 2
			}
			agentTimeout = parsed
		case "--spec":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--spec requires a spec id")
				return 2
			}
			i++
			singleSpec = args[i]
		case "--resume":
			resumeMode = true
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) < 1 {
		fmt.Fprintln(stderr, "build requires a run id")
		fmt.Fprintln(stderr, "usage: dft build <run-id> [--spec <id>] [--resume]")
		return 2
	}
	if len(positional) > 1 && positional[0] == "status" {
		return printBuildStatus(positional[1], stdout, stderr)
	}

	runID := positional[0]

	// Read solution design
	solutionDesign, err := loadSolutionDesign(runID)
	if err != nil {
		fmt.Fprintf(stderr, "load solution design: %v\n", err)
		return 2
	}

	// Create adapter (stub by default; use --adapter copilot for real runs)
	adapter, err := selectAgentAdapter(adapterName, copilotBinary, runID, agentTimeout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	// Build executor registry
	registry := orchv2.NewExecutorRegistry()
	registry.Register("stub", func() orchv2.Executor { return &orchv2.StubExecutor{} })
	registry.Register("speckit", func() orchv2.Executor { return orchv2.NewSpeckitExecutor(".dft/worktrees") })

	// Select specs to execute
	specs := solutionDesign.WBS.Specs
	if singleSpec != "" {
		filtered := make([]domain.SpecRef, 0, 1)
		for _, spec := range specs {
			if spec.ID == singleSpec {
				filtered = append(filtered, spec)
				break
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(stderr, "spec %q not found in WBS\n", singleSpec)
			return 2
		}
		specs = filtered
	}

	// If resuming, read prior result and find first incomplete spec
	if resumeMode {
		prev, err := loadOrchestrationResult(runID)
		if err != nil {
			fmt.Fprintf(stderr, "cannot resume: no prior orchestration result: %v\n", err)
			return 2
		}
		completed := make(map[string]bool)
		for _, r := range prev.SpecResults {
			if r.Status == "completed" {
				completed[r.SpecID] = true
			}
		}
		var remaining []domain.SpecRef
		for _, spec := range specs {
			if !completed[spec.ID] {
				remaining = append(remaining, spec)
			}
		}
		if len(remaining) == 0 {
			fmt.Fprintf(stdout, "all specs already completed for run %s\n", runID)
			return 0
		}
		specs = remaining
	}

	// Dispatch
	dispatcher := orchv2.NewDispatcher(registry)
	result, err := dispatcher.Dispatch(context.Background(), orchv2.DispatcherInput{
		Specs:           specs,
		LaneAssignments: solutionDesign.LaneAssignments,
		RunID:           runID,
		IncrementBranch: "increment/" + runID,
		Registry:        registry,
		Agent:           adapter,
		ArtifactRoot:    ".",
	})
	if err != nil {
		// Save partial result
		if len(result.SpecResults) > 0 {
			_ = writeOrchestrationResult(runID, result)
		}
		fmt.Fprintf(stderr, "build failed: %v\n", err)
		return 2
	}

	// Build artifact manifest from spec results
	artifacts := make([]domain.ArtifactRef, 0, len(result.SpecResults))
	for _, sr := range result.SpecResults {
		artifacts = append(artifacts, sr.Artifacts...)
	}
	result.ArtifactManifest = domain.ArtifactManifest{
		DemandPackageID: runID,
		Artifacts:       artifacts,
	}

	// Save result
	if err := writeOrchestrationResult(runID, result); err != nil {
		fmt.Fprintf(stderr, "save orchestration result: %v\n", err)
		return 2
	}

	// Write to state store
	manifest := domain.RunManifest{ID: runID, Status: domain.RunRunning, Adapter: adapterName}
	store := state.JSONStore{RootDir: "."}
	sqlStore, err := state.OpenSQLiteStore(filepath.Join(".dft", "state.db"))
	if err == nil {
		defer sqlStore.Close()
		_ = saveRunState(store, sqlStore, manifest)
	}

	completed := 0
	failed := 0
	for _, r := range result.SpecResults {
		if r.Status == "completed" {
			completed++
		} else {
			failed++
		}
	}
	fmt.Fprintf(stdout, "%d/%d specs complete", completed, completed+failed)
	if failed > 0 {
		fmt.Fprintf(stdout, ", %d failed", failed)
	}
	fmt.Fprintln(stdout)
	return 0
}

func loadSolutionDesign(runID string) (domainv2.SolutionDesign, error) {
	path := filepath.Join(".dft", "runs", runID, "design", "solution-design.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return domainv2.SolutionDesign{}, fmt.Errorf("read solution design: %w (run the solution phase first)", err)
	}
	var sd domainv2.SolutionDesign
	if err := json.Unmarshal(content, &sd); err != nil {
		return domainv2.SolutionDesign{}, fmt.Errorf("parse solution design: %w", err)
	}
	if err := sd.Validate(); err != nil {
		return domainv2.SolutionDesign{}, fmt.Errorf("invalid solution design: %w", err)
	}
	return sd, nil
}

func writeOrchestrationResult(runID string, result domainv2.OrchestrationResult) error {
	result.CompletedAt = time.Now()
	path := filepath.Join(".dft", "runs", runID, "orchestration-result.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

func loadOrchestrationResult(runID string) (domainv2.OrchestrationResult, error) {
	path := filepath.Join(".dft", "runs", runID, "orchestration-result.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return domainv2.OrchestrationResult{}, fmt.Errorf("read orchestration result: %w", err)
	}
	var result domainv2.OrchestrationResult
	if err := json.Unmarshal(content, &result); err != nil {
		return domainv2.OrchestrationResult{}, fmt.Errorf("parse orchestration result: %w", err)
	}
	return result, nil
}

func printBuildStatus(runID string, stdout io.Writer, stderr io.Writer) int {
	result, err := loadOrchestrationResult(runID)
	if err != nil {
		fmt.Fprintf(stderr, "no build status for run %s: %v\n", runID, err)
		return 2
	}
	for _, r := range result.SpecResults {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", r.SpecID, r.Lane, r.Status)
	}
		return 0
}