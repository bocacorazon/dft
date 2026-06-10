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
	"github.com/bocacorazon/dft/internal/adapters/hermes"
	"github.com/bocacorazon/dft/internal/adapters/state"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
	"github.com/bocacorazon/dft/internal/orchestration"
	"github.com/bocacorazon/dft/internal/ports"
)

const helpText = `dft is a headless workflow engine for spec-driven software production.

Usage:
  dft <command> [arguments]

Commands:
  build         Execute one frozen spec flow from explicit runtime inputs
  evaluate      Execute evaluation phase: readiness → BDD eval → verdict
  eval-harness  Run an automated E2E model evaluation pipeline
  submit        Execute a WBS DAG by dispatching frozen spec flows
  status    Show current or historical run status
  inspect   Inspect run artifacts and step output
  cancel    Cancel a running job
  resume    Resume an interrupted job
  init      Provision dft assets in a target repository
  sync      Update provisioned dft assets
  schema    Output JSON Schema for a contract type (e.g. increment-package)
  validate  Validate a JSON file against a contract type
  help      Show this help text
`

// Run executes the command-line entry point and returns a process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, helpText)
		return 0
	}

	command := strings.TrimSpace(args[0])
	if command == "build" {
		return runBuild(args[1:], stdout, stderr)
	}
	if command == "evaluate" {
		return runEvaluate(args[1:], stdout, stderr)
	}
	if command == "eval-harness" {
		return runEvalHarness(args[1:], stdout, stderr)
	}
	if command == "submit" {
		return runSubmit(args[1:], stdout, stderr)
	}
	if command == "status" {
		return showStatusV2(stdout, stderr)
	}
	if command == "inspect" {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "inspect requires a run id")
			return 2
		}
		return printInspectV2(args[1], stdout, stderr)
	}
	if command == "cancel" {
		return updateRunStatusV2(args[1:], domain.RunCancelled, stdout, stderr)
	}
	if command == "resume" {
		return runResume(args[1:], stdout, stderr)
	}
	if command == "init" || command == "sync" {
		return provisionAssets(command, args[1:], stdout, stderr)
	}
	if command == "schema" {
		return runSchema(args[1:], stdout, stderr)
	}
	if command == "validate" {
		return runValidate(args[1:], stdout, stderr)
	}

	fmt.Fprintf(stderr, "unknown command %q\n\n", command)
	fmt.Fprint(stderr, helpText)
	return 2
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
			Env:           []string{"COPILOT_MACRO_FLOW=true"},
		}, nil
	case "hermes":
		return hermes.Adapter{
			Binary:        copilotBinary,
			Cwd:           ".",
			TranscriptDir: filepath.Join(".dft", "runs", runID, "transcripts"),
			Timeout:       agentTimeout,
			Env:           []string{"HERMES_MACRO_FLOW=true"},
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
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", manifest.ID, manifest.Status, manifest.RawIncrement)
		for _, summary := range loadSpecLaneSummaries(manifest.ID) {
			fmt.Fprintf(stdout, "lane/%s\tlatest_success=%s\tblocked=%s\tauto_resume=%t\trecommendation=%s\n", summary.SpecID, summary.LatestSuccessfulStage, summary.BlockingStage, summary.AutomaticResumeSafe, summary.ResumeRecommendation)
		}
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

func runResume(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "resume requires run id")
		return 2
	}
	store := state.JSONStore{RootDir: "."}
	manifest, err := loadRunManifest(args[0], store)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	spec, err := loadResumableSpecForRun(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	adapter, err := selectAgentAdapter(manifest.Adapter, "", manifest.ID, 30*time.Minute)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	dispatcher, ok := adapter.(ports.CommandDispatcher)
	if !ok {
		fmt.Fprintf(stderr, "adapter %q does not support workflow command resume\n", manifest.Adapter)
		return 2
	}
	worktree := specWorktreeForRun(args[0], spec.ID)
	runner := flow.Runner{
		Agent:            adapter,
		Dispatcher:       dispatcher,
		ArtifactRoot:     ".",
		RunID:            manifest.ID,
		Verifier:         verify.Checker{RootDir: "."},
		CommitLocalSteps: specWorktreeHasGit(worktree.WorktreePath),
		AutoApproveGates: true,
	}
	decision, _, err := orchestration.ResumeSpecKitLane(context.Background(), ".", manifest.ID, spec, worktree, runner)
	sqlStore, openErr := state.OpenSQLiteStore(filepath.Join(".dft", "state.db"))
	if openErr != nil {
		fmt.Fprintln(stderr, openErr)
		return 2
	}
	defer sqlStore.Close()
	if err != nil {
		manifest.Status = domain.RunFailed
		if stateErr := saveRunState(store, sqlStore, manifest); stateErr != nil {
			fmt.Fprintf(stderr, "record failure state: %v\n", stateErr)
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	manifest.Status = domain.RunSucceeded
	if err := saveRunState(store, sqlStore, manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if decision.Completed {
		fmt.Fprintf(stdout, "%s\t%s\t%s\tcomplete\n", manifest.ID, manifest.Status, spec.ID)
		return 0
	}
	fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", manifest.ID, manifest.Status, spec.ID, decision.Stage)
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

func recordFailedRun(jsonStore state.JSONStore, sqlStore *state.SQLiteStore, jobID string, manifest domain.RunManifest) error {
	if err := sqlStore.SetJobStatus(jobID, domain.JobFailed); err != nil {
		return err
	}
	if err := saveRunState(jsonStore, sqlStore, manifest); err != nil {
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

func loadResumableSpecForRun(runID string) (domain.SpecRef, error) {
	wbs, err := loadWBSForRun(runID)
	if err != nil {
		return domain.SpecRef{}, err
	}
	if len(wbs.Specs) == 1 {
		return wbs.Specs[0], nil
	}
	specsByID := make(map[string]domain.SpecRef, len(wbs.Specs))
	for _, spec := range wbs.Specs {
		specsByID[spec.ID] = spec
	}
	summaries := loadSpecLaneSummaries(runID)
	var candidate *domain.SpecRef
	for _, summary := range summaries {
		spec, ok := specsByID[summary.SpecID]
		if !ok {
			continue
		}
		if summary.BlockingStage == "" && summary.ResumeRecommendation == "" && summary.LatestSuccessfulStage == orchestration.SpecKitStageMergeback {
			continue
		}
		if candidate != nil && candidate.ID != spec.ID {
			return domain.SpecRef{}, fmt.Errorf("resume found multiple active specs in run %s", runID)
		}
		copy := spec
		candidate = &copy
	}
	if candidate == nil {
		return domain.SpecRef{}, fmt.Errorf("resume could not identify an active spec in run %s", runID)
	}
	return *candidate, nil
}

func loadWBSForRun(runID string) (domain.WBS, error) {
	content, err := os.ReadFile(filepath.Join(".dft", "runs", runID, "design", "wbs.json"))
	if err != nil {
		return domain.WBS{}, fmt.Errorf("read run WBS: %w", err)
	}
	var wbs domain.WBS
	if err := json.Unmarshal(content, &wbs); err != nil {
		return domain.WBS{}, fmt.Errorf("parse run WBS: %w", err)
	}
	return wbs, nil
}

func specWorktreeForRun(runID string, specID string) orchestration.SpecWorktree {
	return orchestration.SpecWorktree{
		RunID:           runID,
		SpecID:          specID,
		Branch:          "spec/" + runID + "/" + specID,
		IncrementBranch: "increment/" + runID,
		WorktreePath:    filepath.Join(".dft", "worktrees", runID, specID),
		SpecKitEnv: map[string]string{
			"GIT_BRANCH_NAME": orchestration.SpecKitFeatureBranchName(specID),
		},
	}
}

func specWorktreeHasGit(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func loadSpecLaneSummaries(runID string) []orchestration.SpecKitLaneSummary {
	wbs, err := loadWBSForRun(runID)
	if err != nil {
		return nil
	}
	summaries := make([]orchestration.SpecKitLaneSummary, 0, len(wbs.Specs))
	for _, spec := range wbs.Specs {
		worktree := specWorktreeForRun(runID, spec.ID)
		summary, err := orchestration.SummarizeSpecKitLane(".", runID, spec, worktree)
		if err != nil {
			continue
		}
		if summary.SpecID == "" {
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries
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
	for _, summary := range loadSpecLaneSummaries(runID) {
		fmt.Fprintf(stdout, "lane/%s\tlatest_success=%s\tblocked=%s\tauto_resume=%t\trecommendation=%s\n", summary.SpecID, summary.LatestSuccessfulStage, summary.BlockingStage, summary.AutomaticResumeSafe, summary.ResumeRecommendation)
		if len(summary.LatestFindingsSummary) > 0 {
			fmt.Fprintf(stdout, "lane/%s/findings\t%v\n", summary.SpecID, summary.LatestFindingsSummary)
		}
	}
	return 0
}
