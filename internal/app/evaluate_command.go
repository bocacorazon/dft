package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bocacorazon/dft/internal/domain"
	dftEval "github.com/bocacorazon/dft/internal/eval"
)

func runEvaluate(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evaluate requires a run id")
		fmt.Fprintln(stderr, "usage: dft evaluate <run-id>")
		return 2
	}
	if args[0] == "inspect" {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "evaluate inspect requires a run id")
			return 2
		}
		return printEvalResult(args[1], stdout, stderr)
	}

	runID := args[0]

	// Read solution design for eval surface contract
	sd, err := loadSolutionDesign(runID)
	if err != nil {
		fmt.Fprintf(stderr, "load solution design: %v\n", err)
		return 2
	}

	// Read orchestration result for artifact manifest
	orchResult, err := loadOrchestrationResult(runID)
	if err != nil {
		fmt.Fprintf(stderr, "load orchestration result: %v\n", err)
		return 2
	}

	// Build eval input from contracts
	evalInput := dftEval.AuthorInput{
		IncrementPackage:    domain.IncrementPackage{ID: runID},
		SurfaceContract:  sd.EvalSurfaceContract,
		ArtifactManifest: orchResult.ArtifactManifest,
		StepCatalog:      dftEval.DefaultStepCatalog(),
	}

	// Run eval orchestrator
	orchestrator := dftEval.Orchestrator{
		ArtifactRoot: ".",
		RunID:        runID,
	}
	result, err := orchestrator.Run(context.Background(), evalInput)
	if err != nil {
		fmt.Fprintf(stderr, "evaluation failed: %v\n", err)
		return 2
	}

	// Write verdict
	evalPath := filepath.Join(".dft", "runs", runID, "eval", "evaluation.json")
	if err := os.MkdirAll(filepath.Dir(evalPath), 0o755); err != nil {
		fmt.Fprintf(stderr, "create eval dir: %v\n", err)
		return 2
	}
	evalJSON := map[string]any{
		"verdict":   result.Result.Status,
		"coverage":  result.Result.Coverage,
		"findings":  result.Result.Findings,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	content, _ := json.MarshalIndent(evalJSON, "", "  ")
	os.WriteFile(evalPath, append(content, '\n'), 0o644)

	fmt.Fprintf(stdout, "verdict=%s", result.Result.Status)
	if result.Result.Coverage.Total > 0 {
		fmt.Fprintf(stdout, " coverage=%d/%d", result.Result.Coverage.Covered, result.Result.Coverage.Total)
	}
	if len(result.Result.Findings) > 0 {
		fmt.Fprintf(stdout, " findings=%d", len(result.Result.Findings))
	}
	fmt.Fprintln(stdout)
	return 0
}

func printEvalResult(runID string, stdout io.Writer, stderr io.Writer) int {
	path := filepath.Join(".dft", "runs", runID, "eval", "evaluation.json")
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "no eval result for run %s: %v\n", runID, err)
		return 2
	}
	fmt.Fprint(stdout, string(content))
	return 0
}