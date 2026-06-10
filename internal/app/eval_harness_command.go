package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bocacorazon/dft/internal/eval/harness"
)

func runEvalHarness(args []string, stdout io.Writer, stderr io.Writer) int {
	var increment string
	var agentType string = "mock"
	var agentModel string
	var agentProfile string
	var executor string = "speckit"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--increment":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--increment requires a value")
				return 2
			}
			i++
			increment = args[i]
		case "--agent":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--agent requires a value (hermes or mock)")
				return 2
			}
			i++
			agentType = args[i]
		case "--model":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--model requires a value")
				return 2
			}
			i++
			agentModel = args[i]
		case "--profile":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--profile requires a value")
				return 2
			}
			i++
			agentProfile = args[i]
		case "--executor":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "--executor requires a value (speckit, stub, copilot)")
				return 2
			}
			i++
			executor = args[i]
		default:
			if strings.HasPrefix(args[i], "--") {
				fmt.Fprintf(stderr, "unknown flag %q\n", args[i])
				return 2
			}
			// If not a flag, treat as increment text if not already set via --increment
			if increment == "" {
				increment = args[i]
			}
		}
	}

	if increment == "" {
		fmt.Fprintln(stderr, "eval-harness requires increment text (via --increment or positional arguments)")
		return 2
	}

	dftBinary, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "failed to get dft executable path: %v\n", err)
		return 1
	}

	var designAgent harness.Agent
	if agentType == "hermes" {
		designAgent = &harness.HermesAgent{
			Profile: agentProfile,
			Model:   agentModel,
		}
	} else if agentType == "mock" {
		designAgent = &harness.MockAgent{}
	} else {
		fmt.Fprintf(stderr, "unknown agent type %q. Supported: hermes, mock\n", agentType)
		return 2
	}

	h := harness.New(dftBinary, designAgent, executor)

	fmt.Fprintf(stdout, "Starting eval-harness run...\n")
	fmt.Fprintf(stdout, "  Agent:   %s\n", designAgent.Name())
	if agentModel != "" {
		fmt.Fprintf(stdout, "  Model:   %s\n", agentModel)
	}
	fmt.Fprintf(stdout, "  Increment:  %s\n", increment)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	result, err := h.Run(ctx, increment)
	if err != nil {
		fmt.Fprintf(stderr, "harness execution failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "\n--- Run Complete ---\n")
	fmt.Fprintf(stdout, "Run ID:     %s\n", result.RunID)
	fmt.Fprintf(stdout, "Workspace:  %s\n", result.WorkspaceDir)
	fmt.Fprintf(stdout, "Time:       %v\n", result.TotalTime)

	if result.DesignError != "" {
		fmt.Fprintf(stdout, "Design:     FAILED - %s\n", result.DesignError)
	} else {
		fmt.Fprintf(stdout, "Design:     OK\n")
	}

	if !result.BuildOK {
		fmt.Fprintf(stdout, "Build:      FAILED\n")
		fmt.Fprintf(stdout, "Build Output: %s\n", result.BuildOutput)
	} else {
		fmt.Fprintf(stdout, "Build:      OK\n")
	}

	fmt.Fprintf(stdout, "Evaluation: %s\n", result.EvalVerdict)
	if result.EvalOutput != "" {
		fmt.Fprintf(stdout, "Eval Output: %s\n", result.EvalOutput)
	}

	// Dump JSON result for programmatic parsing
	fmt.Fprintf(stdout, "\nJSON Report:\n")
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Fprintln(stdout, string(jsonBytes))

	if result.EvalVerdict != "pass" {
		return 1
	}
	return 0
}
