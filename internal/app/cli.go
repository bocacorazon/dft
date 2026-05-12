package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/copilot"
	gitadapter "github.com/bocacorazon/dft/internal/adapters/git"
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
`

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

	fmt.Fprintf(stderr, "unknown command %q\n\n", command)
	fmt.Fprint(stderr, helpText)
	return 2
}

func runSubmit(args []string, stdout io.Writer, stderr io.Writer) int {
	adapterName := "stub"
	dogfood := false
	copilotBinary := ""
	dryRun := false
	holdIncrement := false
	evalRetries := 1
	agentTimeout := 30 * time.Minute
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
			dryRun = true
		case "--dogfood":
			dogfood = true
		case "--hold-increment", "--no-merge":
			holdIncrement = true
		case "--eval-retries":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--eval-retries requires a value")
				return 2
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed < 0 {
				fmt.Fprintln(stderr, "--eval-retries requires a non-negative integer")
				return 2
			}
			evalRetries = parsed
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
		default:
			demandParts = append(demandParts, args[i])
		}
	}

	runID := os.Getenv("DFT_RUN_ID")
	if runID == "" {
		runID = "run-" + time.Now().UTC().Format("20060102-150405")
	}
	adapter, err := selectAgentAdapter(adapterName, copilotBinary, runID, agentTimeout)
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
	sqlStore, err := state.OpenSQLiteStore(filepath.Join(".dft", "state.db"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer sqlStore.Close()
	if err := saveRunState(store, sqlStore, manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	jobID := "job-" + runID
	if err := sqlStore.Enqueue(jobID, runID); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := sqlStore.SetJobStatus(jobID, domain.JobRunning); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if dogfood {
		if err := runDogfoodLoop(context.Background(), demandPackage, adapter, dryRun, holdIncrement, evalRetries); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		manifest.Status = domain.RunSucceeded
		if err := sqlStore.SetJobStatus(jobID, domain.JobDone); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := saveRunState(store, sqlStore, manifest); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		fmt.Fprintf(stdout, "dogfood loop complete for run %s\n", runID)
		return 0
	}
	manifest.Status = domain.RunSucceeded
	if err := sqlStore.SetJobStatus(jobID, domain.JobDone); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := saveRunState(store, sqlStore, manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	fmt.Fprintf(stdout, "created demand package %s for run %s\n", demandPackage.ID, runID)
	return 0
}

func selectAgentAdapter(name string, copilotBinary string, runID string, agentTimeout time.Duration) (ports.AgentAdapter, error) {
	switch name {
	case "stub":
		return agentstub.Adapter{}, nil
	case "copilot":
		return copilot.Adapter{
			Binary:        copilotBinary,
			Cwd:           ".",
			TranscriptDir: filepath.Join(".dft", "runs", runID, "transcripts"),
			Timeout:       agentTimeout,
		}, nil
	default:
		return nil, fmt.Errorf("unknown adapter %q", name)
	}
}

func runStatus(stdout io.Writer, stderr io.Writer) int {
	if sqlStore, err := state.OpenSQLiteStore(filepath.Join(".dft", "state.db")); err == nil {
		defer sqlStore.Close()
		manifests, err := sqlStore.List()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return printManifests(manifests, stdout)
	}
	manifests, err := (state.JSONStore{RootDir: "."}).List()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return printManifests(manifests, stdout)
}

func printManifests(manifests []domain.RunManifest, stdout io.Writer) int {
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
	if code := walkRunArtifacts(runDir, stdout, stderr); code != 0 {
		return code
	}
	return printDurableRunDetails(args[0], stdout, stderr)
}

func updateRunStatus(args []string, status domain.RunStatus, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "%s requires run id\n", status)
		return 2
	}
	store := state.JSONStore{RootDir: "."}
	manifest, err := loadRunManifest(args[0], store)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	manifest.Status = status
	sqlStore, err := state.OpenSQLiteStore(filepath.Join(".dft", "state.db"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer sqlStore.Close()
	if err := saveRunState(store, sqlStore, manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "%s\t%s\n", manifest.ID, manifest.Status)
	return 0
}

func saveRunState(jsonStore state.JSONStore, sqlStore *state.SQLiteStore, manifest domain.RunManifest) error {
	if err := jsonStore.Save(manifest); err != nil {
		return err
	}
	if err := sqlStore.Save(manifest); err != nil {
		return err
	}
	return nil
}

func loadRunManifest(id string, jsonStore state.JSONStore) (domain.RunManifest, error) {
	if sqlStore, err := state.OpenSQLiteStore(filepath.Join(".dft", "state.db")); err == nil {
		defer sqlStore.Close()
		return sqlStore.Load(id)
	}
	return jsonStore.Load(id)
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

func printDurableRunDetails(runID string, stdout io.Writer, stderr io.Writer) int {
	sqlStore, err := state.OpenSQLiteStore(filepath.Join(".dft", "state.db"))
	if err != nil {
		return 0
	}
	defer sqlStore.Close()
	steps, err := sqlStore.ListSteps(runID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, step := range steps {
		fmt.Fprintf(stdout, "state/steps/%s\t%s\t%s\n", step.StepID, step.Status, step.Commit)
	}
	entries, err := sqlStore.ListInboxEntries(runID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, entry := range entries {
		fmt.Fprintf(stdout, "inbox/%s\t%s\t%s\n", entry.ID, entry.Status, entry.Message)
	}
	return 0
}

func runDogfoodLoop(ctx context.Context, demandPackage domain.DemandPackage, adapter ports.AgentAdapter, dryRun bool, holdIncrement bool, evalRetries int) error {
	gitPort := ports.GitPort(gitadapter.Adapter{RepoDir: "."})
	if dryRun {
		gitPort = dryRunGit{defaultBranch: "main"}
	}
	if _, err := (orchestration.MacroOrchestrator{
		Agent: adapter,
		Worktrees: orchestration.WorktreeManager{
			Git:          gitPort,
			WorktreeRoot: filepath.Join(".dft", "worktrees"),
		},
		ArtifactRoot:     ".",
		Verifier:         verify.Checker{RootDir: "."},
		CommitLocalSteps: !dryRun,
		HoldIncrement:    holdIncrement,
		MaxEvalRetries:   evalRetries,
	}).Execute(ctx, demandPackage); err != nil {
		return fmt.Errorf("execute dogfood macro loop: %w", err)
	}
	runner := flow.Runner{Agent: adapter, ArtifactRoot: ".", RunID: demandPackage.ID, CommitLocalSteps: !dryRun}
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
