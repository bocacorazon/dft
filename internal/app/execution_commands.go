package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	gitadapter "github.com/bocacorazon/dft/internal/adapters/git"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/execution"
)

func runBuild(args []string, stdout io.Writer, stderr io.Writer) int {
	adapterName := "stub"
	copilotBinary := ""
	agentTimeout := 30 * time.Minute
	request := domain.ExecutionRequest{}

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
		case "--prompt":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--prompt requires a value")
				return 2
			}
			i++
			request.Prompt = args[i]
		case "--prompt-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--prompt-file requires a path")
				return 2
			}
			i++
			request.PromptPath = args[i]
		case "--workflow":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--workflow requires a value")
				return 2
			}
			i++
			request.Workflow = args[i]
		case "--flow":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--flow requires a path")
				return 2
			}
			i++
			request.WorkflowPath = args[i]
		case "--feature-slug":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--feature-slug requires a value")
				return 2
			}
			i++
			request.FeatureSlug = args[i]
		case "--increment-branch":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--increment-branch requires a value")
				return 2
			}
			i++
			request.IncrementBranch = args[i]
		case "--models":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--models requires a path")
				return 2
			}
			i++
			request.ModelConfigPath = args[i]
		case "--model-family":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--model-family requires a value")
				return 2
			}
			i++
			request.ModelFamily = args[i]
		default:
			fmt.Fprintf(stderr, "unknown build argument %q\n", args[i])
			return 2
		}
	}

	if err := request.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	runID := executionRunID("build", request.FeatureSlug)
	adapter, err := selectAgentAdapter(adapterName, copilotBinary, runID, agentTimeout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	git := gitadapter.Adapter{RepoDir: "."}
	service := execution.Service{
		Agent:        adapter,
		Git:          git,
		ArtifactRoot: ".",
	}
	result, err := service.Execute(context.Background(), runID, request.FeatureSlug, request)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "completed spec %s in run %s\n", result.FeatureSlug, result.RunID)
	return 0
}

func runSubmit(args []string, stdout io.Writer, stderr io.Writer) int {
	adapterName := "stub"
	copilotBinary := ""
	agentTimeout := 30 * time.Minute
	request := domain.OrchestrationRequest{}

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
		case "--wbs":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--wbs requires a path")
				return 2
			}
			i++
			request.WBSPath = args[i]
		case "--workflow":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--workflow requires a value")
				return 2
			}
			i++
			request.Workflow = args[i]
		case "--flow":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--flow requires a path")
				return 2
			}
			i++
			request.WorkflowPath = args[i]
		case "--base-branch":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--base-branch requires a value")
				return 2
			}
			i++
			request.BaseBranch = args[i]
		case "--increment-branch":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--increment-branch requires a value")
				return 2
			}
			i++
			request.IncrementBranch = args[i]
		case "--models":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--models requires a path")
				return 2
			}
			i++
			request.ModelConfigPath = args[i]
		case "--model-family":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--model-family requires a value")
				return 2
			}
			i++
			request.ModelFamily = args[i]
		case "--callback-url":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--callback-url requires a value")
				return 2
			}
			i++
			request.CompletionURL = args[i]
		default:
			fmt.Fprintf(stderr, "unknown submit argument %q\n", args[i])
			return 2
		}
	}

	if err := request.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	runID := executionRunID("submit", filepath.Base(request.WBSPath))
	adapter, err := selectAgentAdapter(adapterName, copilotBinary, runID, agentTimeout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	git := gitadapter.Adapter{RepoDir: "."}
	service := execution.Service{
		Agent:        adapter,
		Git:          git,
		ArtifactRoot: ".",
	}
	registry := execution.NewRegistry(service)
	registry.Register(execution.WorktreeLane, execution.WorktreeExecutor{
		Agent:        adapter,
		Git:          git,
		ArtifactRoot: ".",
	})
	orchestrator := execution.Orchestrator{
		Registry:     registry,
		Git:          git,
		ArtifactRoot: ".",
	}
	result, err := orchestrator.Execute(context.Background(), runID, request)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintf(stdout, "%d/%d specs complete for run %s\n", len(result.SpecResults), len(result.SpecResults), result.RunID)
	return 0
}

func executionRunID(prefix string, value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "run"
	}
	return fmt.Sprintf("%s-%s-%s", prefix, slug, time.Now().UTC().Format("20060102-150405"))
}
