package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/copilot"
	"github.com/bocacorazon/dft/internal/adapters/state"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
	"github.com/bocacorazon/dft/internal/intake"
	"github.com/bocacorazon/dft/internal/orchestration"
	"github.com/bocacorazon/dft/internal/ports"
	"github.com/bocacorazon/dft/internal/review"
)

const helpText = `dft is a headless workflow engine for spec-driven software production.

Usage:
  dft <command> [arguments]

Commands:
  submit    Start an increment from a demand package request
  status    Show current or historical run status
  inspect   Inspect run artifacts and step output
  cancel    Cancel a running job
  resume    Resume an interrupted job
  init      Provision dft assets in a target repository
  sync      Update provisioned dft assets
  help      Show this help text

The command surface is scaffolded. Non-help commands will fail until their
increment implements the corresponding behavior.
`

var plannedCommands = map[string]struct{}{
	"submit":  {},
	"status":  {},
	"inspect": {},
	"cancel":  {},
	"resume":  {},
	"init":    {},
	"sync":    {},
}

// Run executes the command-line entry point and returns a process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, helpText)
		return 0
	}

	command := strings.TrimSpace(args[0])
	if command == "submit" {
		return runSubmit(args[1:], stdout, stderr)
	}
	if command == "status" {
		return runStatus(stdout, stderr)
	}
	if command == "inspect" {
		return runInspect(args[1:], stdout, stderr)
	}
	if command == "cancel" {
		return updateRunStatus(args[1:], domain.RunCancelled, stdout, stderr)
	}
	if command == "resume" {
		return updateRunStatus(args[1:], domain.RunRunning, stdout, stderr)
	}
	if command == "init" || command == "sync" {
		return provisionAssets(command, stdout, stderr)
	}
	if _, ok := plannedCommands[command]; ok {
		fmt.Fprintf(stderr, "dft %s is not implemented yet\n", command)
		return 2
	}

	fmt.Fprintf(stderr, "unknown command %q\n\n", command)
	fmt.Fprint(stderr, helpText)
	return 2
}

func runSubmit(args []string, stdout io.Writer, stderr io.Writer) int {
	adapterName := "stub"
	dogfood := false
	copilotBinary := ""
	var demandParts []string

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
		case "--dry-run":
			// Accepted for compatibility. Local submit is safe by default and
			// records all mutations under .dft/.
		case "--dogfood":
			dogfood = true
		default:
			demandParts = append(demandParts, args[i])
		}
	}

	runID := os.Getenv("DFT_RUN_ID")
	if runID == "" {
		runID = "run-" + time.Now().UTC().Format("20060102-150405")
	}
	adapter, err := selectAgentAdapter(adapterName, copilotBinary, runID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	service := intake.Service{
		Adapter: adapter,
		RunID:   runID,
		RootDir: ".",
	}
	demandPackage, err := service.CreateDemandPackage(context.Background(), strings.Join(demandParts, " "))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	store := state.JSONStore{RootDir: "."}
	manifest := domain.RunManifest{ID: runID, Status: domain.RunRunning, Adapter: adapterName, RawDemand: demandPackage.RawDemand}
	if err := store.Save(manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if dogfood {
		if err := runDogfoodLoop(context.Background(), demandPackage); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		manifest.Status = domain.RunSucceeded
		if err := store.Save(manifest); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		fmt.Fprintf(stdout, "dogfood loop complete for run %s\n", runID)
		return 0
	}

	manifest.Status = domain.RunSucceeded
	if err := store.Save(manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	fmt.Fprintf(stdout, "created demand package %s for run %s\n", demandPackage.ID, runID)
	return 0
}

func selectAgentAdapter(name string, copilotBinary string, runID string) (ports.AgentAdapter, error) {
	switch name {
	case "stub":
		return agentstub.Adapter{}, nil
	case "copilot":
		return copilot.Adapter{
			Binary:        copilotBinary,
			Cwd:           ".",
			TranscriptDir: filepath.Join(".dft", "runs", runID, "transcripts"),
			Timeout:       10 * time.Minute,
		}, nil
	default:
		return nil, fmt.Errorf("unknown adapter %q", name)
	}
}

func runStatus(stdout io.Writer, stderr io.Writer) int {
	manifests, err := (state.JSONStore{RootDir: "."}).List()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(manifests) == 0 {
		fmt.Fprintln(stdout, "no runs")
		return 0
	}
	for _, manifest := range manifests {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", manifest.ID, manifest.Status, manifest.RawDemand)
	}
	return 0
}

func runInspect(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "inspect requires run id")
		return 2
	}
	runDir := filepath.Join(".dft", "runs", args[0])
	if _, err := os.Stat(runDir); err != nil {
		fmt.Fprintf(stderr, "run %s not found: %v\n", args[0], err)
		return 2
	}
	return walkRunArtifacts(runDir, stdout, stderr)
}

func updateRunStatus(args []string, status domain.RunStatus, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "%s requires run id\n", status)
		return 2
	}
	store := state.JSONStore{RootDir: "."}
	manifest, err := store.Load(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	manifest.Status = status
	if err := store.Save(manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "%s\t%s\n", manifest.ID, manifest.Status)
	return 0
}

func walkRunArtifacts(runDir string, stdout io.Writer, stderr io.Writer) int {
	err := filepath.WalkDir(runDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(runDir, path)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, relative)
		return nil
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func provisionAssets(command string, stdout io.Writer, stderr io.Writer) int {
	assets := map[string]string{
		filepath.Join(".dft", "agents", "dft-intake.agent.md"): `---
description: Normalize raw user demand into a demand package.
---

# dft Intake Agent

Return strict JSON for a demand package.
`,
		filepath.Join(".dft", "flows", "spec-lane.json"): `{
  "max_spec_parallelism": 1,
  "steps": [
    {
      "id": "implement",
      "type": "agent",
      "agent_name": "dft-intake.agent.md",
      "prompt": "Execute the spec lane",
      "max_iterations": 1
    }
  ]
}
`,
		filepath.Join(".dft", "context", "constitution.md"): "# dft context\n\nFollow repository constitution and mandatory TDD.\n",
	}
	for path, content := range assets {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if command == "init" {
			if _, err := os.Stat(path); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				fmt.Fprintln(stderr, err)
				return 2
			}
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	fmt.Fprintf(stdout, "dft %s complete\n", command)
	return 0
}

func runDogfoodLoop(ctx context.Context, demandPackage domain.DemandPackage) error {
	stub := agentstub.Adapter{}
	if _, err := (orchestration.MacroOrchestrator{
		Agent: stub,
		Worktrees: orchestration.WorktreeManager{
			Git:          dryRunGit{defaultBranch: "main"},
			WorktreeRoot: filepath.Join(".dft", "worktrees"),
		},
		ArtifactRoot: ".",
		Verifier:     verify.Checker{RootDir: "."},
		Review:       domain.ReviewDecision{Approved: true},
	}).Execute(ctx, demandPackage); err != nil {
		return fmt.Errorf("execute dogfood macro loop: %w", err)
	}
	runner := flow.Runner{Agent: stub, ArtifactRoot: ".", RunID: demandPackage.ID}
	if _, err := runner.Execute(ctx, flow.Definition{Steps: []flow.Step{{
		ID:        "dogfood-intake",
		Type:      flow.StepAgent,
		AgentName: "dft-intake.agent.md",
		Prompt:    "Generate feedback seed for the next dft increment",
		Demand:    demandPackage.RawDemand,
	}}}); err != nil {
		return fmt.Errorf("run dogfood lane: %w", err)
	}

	evaluator := review.Evaluator{
		Verifier: verify.Checker{RootDir: "."},
		RunID:    demandPackage.ID,
	}
	if _, err := evaluator.Evaluate(ctx, []domain.Check{
		{ID: "wbs", Kind: domain.CheckFileExists, Args: []string{filepath.Join(".dft", "runs", demandPackage.ID, "design", "wbs.json")}},
		{ID: "lane-assignments", Kind: domain.CheckFileExists, Args: []string{filepath.Join(".dft", "runs", demandPackage.ID, "design", "lane-assignments.json")}},
	}); err != nil {
		return fmt.Errorf("evaluate dogfood run: %w", err)
	}

	next := demandPackage
	next.ID = demandPackage.ID + "-next"
	next.RawDemand = "Use dogfood findings to improve: " + demandPackage.RawDemand
	content, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("encode next demand package: %w", err)
	}
	path := filepath.Join(".dft", "runs", demandPackage.ID, "next-demand-package.json")
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write next demand package: %w", err)
	}
	return nil
}

type dryRunGit struct {
	defaultBranch string
}

func (g dryRunGit) DefaultBranch(context.Context) (string, error) {
	return g.defaultBranch, nil
}

func (dryRunGit) CreateBranch(context.Context, ports.CreateBranchRequest) error {
	return nil
}

func (dryRunGit) CreateWorktree(context.Context, ports.CreateWorktreeRequest) error {
	return nil
}

func (dryRunGit) Merge(context.Context, ports.MergeRequest) error {
	return nil
}
