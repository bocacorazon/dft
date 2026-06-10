package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
)

// phaseProgress detects which phases are complete for a run.
type phaseProgress struct {
	Intent   string // "complete", "incomplete", "-"
	Solution string
	Build    string
	Evaluate string
}

func detectPhaseProgress(runID string) phaseProgress {
	runDir := filepath.Join(".dft", "runs", runID)
	p := phaseProgress{Intent: "-", Solution: "-", Build: "-", Evaluate: "-"}

	// Intent: increment-package.json exists and is verified
	if dp, err := loadIncrementPackage(runID); err == nil {
		if dp.VerifiedComplete {
			p.Intent = "complete"
		} else {
			p.Intent = "incomplete"
		}
	}

	// Solution: solution-design.json exists
	if _, err := loadSolutionDesign(runID); err == nil {
		p.Solution = "complete"
	} else if p.Intent == "complete" {
		p.Solution = "incomplete"
	}

	// Build: orchestration-result.json exists
	if or, err := loadOrchestrationResult(runID); err == nil {
		completed := 0
		failed := 0
		for _, r := range or.SpecResults {
			switch r.Status {
			case "completed":
				completed++
			case "failed":
				failed++
			}
		}
		if failed > 0 {
			p.Build = fmt.Sprintf("%d/%d*", completed, completed+failed)
		} else if len(or.SpecResults) > 0 {
			p.Build = fmt.Sprintf("%d/%d", completed, len(or.SpecResults))
		} else {
			p.Build = "running"
		}
	} else if p.Solution == "complete" {
		p.Build = "incomplete"
	}

	// Evaluate: evaluation.json exists
	evalPath := filepath.Join(runDir, "eval", "evaluation.json")
	if _, err := os.Stat(evalPath); err == nil {
		p.Evaluate = "complete"
	} else if p.Build != "-" && p.Build != "incomplete" {
		p.Evaluate = "incomplete"
	}

	return p
}

func loadIncrementPackage(runID string) (domain.IncrementPackage, error) {
	path := filepath.Join(".dft", "runs", runID, "intent", "increment-package.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return domain.IncrementPackage{}, err
	}
	var dp domain.IncrementPackage
	if err := json.Unmarshal(content, &dp); err != nil {
		return domain.IncrementPackage{}, err
	}
	return dp, nil
}

// printPhaseStatus displays v2 phase progress for all runs.
func printPhaseStatus(stdout io.Writer, stderr io.Writer) int {
	entries, err := os.ReadDir(filepath.Join(".dft", "runs"))
	if err != nil {
		fmt.Fprintln(stdout, "no runs")
		return 0
	}
	printed := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		progress := detectPhaseProgress(runID)
		fmt.Fprintf(stdout, "%s\tintent=%s\tsolution=%s\tbuild=%s\teval=%s\n",
			runID, progress.Intent, progress.Solution, progress.Build, progress.Evaluate)
		printed = true
		// Also show per-spec status if build has results
		if progress.Build != "-" && progress.Build != "incomplete" && progress.Build != "running" {
			if or, err := loadOrchestrationResult(runID); err == nil {
				for _, r := range or.SpecResults {
					fmt.Fprintf(stdout, " spec/%s\t%s\t%s\n", r.SpecID, r.Lane, r.Status)
				}
			}
		}
	}
	if !printed {
		fmt.Fprintln(stdout, "no runs")
	}
	return 0
}

// printInspectV2 shows detailed v2 phase information for a run.
func printInspectV2(runID string, stdout io.Writer, stderr io.Writer) int {
	// Walk artifacts first (backward compat with existing tests)
	runDir := filepath.Join(".dft", "runs", runID)
	if code := walkRunArtifacts(runDir, stdout, stderr); code != 0 {
		return code
	}

	progress := detectPhaseProgress(runID)
	fmt.Fprintf(stdout, "run: %s\n", runID)
	fmt.Fprintf(stdout, "  intent:    %s\n", progress.Intent)
	fmt.Fprintf(stdout, "  solution:  %s\n", progress.Solution)
	fmt.Fprintf(stdout, "  build:     %s\n", progress.Build)
	fmt.Fprintf(stdout, "  evaluate:  %s\n", progress.Evaluate)

	// Show contract artifact details
	if dp, err := loadIncrementPackage(runID); err == nil {
		fmt.Fprintf(stdout, "\n--- Increment Package ---\n")
		fmt.Fprintf(stdout, "  title: %s\n", dp.Title)
		fmt.Fprintf(stdout, "  acs: %d\n", len(dp.AcceptanceCriteria))
		fmt.Fprintf(stdout, "  verified: %v\n", dp.VerifiedComplete)
	}

	if sd, err := loadSolutionDesign(runID); err == nil {
		fmt.Fprintf(stdout, "\n--- Solution Design ---\n")
		fmt.Fprintf(stdout, "  specs: %d\n", len(sd.WBS.Specs))
		fmt.Fprintf(stdout, "  scenarios: %d\n", len(sd.TestPlan.Scenarios))
		fmt.Fprintf(stdout, "  surfaces: %d\n", len(sd.EvalSurfaceContract.Surfaces))
		for _, la := range sd.LaneAssignments {
			fmt.Fprintf(stdout, "  lane: %s -> %s (%s)\n", la.SpecID, la.Lane, la.Rationale)
		}
	}

	if or, err := loadOrchestrationResult(runID); err == nil {
		fmt.Fprintf(stdout, "\n--- Build Results ---\n")
		for _, r := range or.SpecResults {
			fmt.Fprintf(stdout, "  %s: %s (%s)\n", r.SpecID, r.Status, r.Lane)
			if len(r.Findings) > 0 {
				for _, f := range r.Findings {
					fmt.Fprintf(stdout, "    finding: [%s] %s\n", f.Severity, f.Message)
				}
			}
		}
	}

	// Show eval result if exists
	evalPath := filepath.Join(".dft", "runs", runID, "eval", "evaluation.json")
	if content, err := os.ReadFile(evalPath); err == nil {
		fmt.Fprintf(stdout, "\n--- Evaluation ---\n")
		fmt.Fprint(stdout, string(content))
	}

	if records, err := loadAgentCalls(runID); err == nil && len(records) > 0 {
		fmt.Fprintf(stdout, "\n--- Agent Stats ---\n")
		printAgentStats(stdout, records)
	}

	return 0
}

// showStatusV2 decides between phase-based and manifest-based status format.
func showStatusV2(stdout io.Writer, stderr io.Writer) int {
	// If any runs have phase artifacts, use the phase format.
	entries, err := os.ReadDir(filepath.Join(".dft", "runs"))
	if err != nil {
		return runStatus(stdout, stderr)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := loadIncrementPackage(entry.Name()); err == nil {
			return printPhaseStatus(stdout, stderr)
		}
		if _, err := loadSolutionDesign(entry.Name()); err == nil {
			return printPhaseStatus(stdout, stderr)
		}
	}
	return runStatus(stdout, stderr)
}

// updateRunStatusV2 also uses the domain.RunManifest for backward compat.
func updateRunStatusV2(args []string, status domain.RunStatus, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "%s requires run id\n", status)
		return 2
	}
	runID := args[0]
	runDir := filepath.Join(".dft", "runs", runID)
	if _, err := os.Stat(runDir); err != nil {
		fmt.Fprintf(stderr, "run %s not found: %v\n", runID, err)
		return 2
	}
	// Write a .status file as a simple marker
	statusPath := filepath.Join(runDir, ".status")
	os.WriteFile(statusPath, []byte(string(status)+"\n"), 0o644)
	fmt.Fprintf(stdout, "%s\t%s\n", runID, status)
	return 0
}
