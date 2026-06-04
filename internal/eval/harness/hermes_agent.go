package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type HermesAgent struct {
	Profile string // Optional hermes profile (e.g. "dft-eval")
	Model   string // Optional model override
}

func (h *HermesAgent) Name() string { return "hermes" }

func (h *HermesAgent) Design(ctx context.Context, demand string, workspaceDir string, designDir string) error {
	prompt := "You are the architecture team for dft (Dark Factory Toolkit).\n" +
		"Your job is to produce two JSON files in the design directory.\n\n" +
		"## OUTPUT FILES (write both to " + designDir + "/)\n\n" +
		"### File 1: demand-package.json\n" +
		"```json\n" +
		"{\n" +
		"  \"id\": \"<run-id>\",\n" +
		"  \"title\": \"<concise title>\",\n" +
		"  \"raw_demand\": \"<original demand>\",\n" +
		"  \"refined_demand\": \"<clear unambiguous statement>\",\n" +
		"  \"acceptance_criteria\": [\n" +
		"    {\"id\": \"AC-001\", \"description\": \"<observable behavior>\"}\n" +
		"  ],\n" +
		"  \"assumptions\": [\"<assumption>\"],\n" +
		"  \"non_goals\": [\"<out of scope>\"],\n" +
		"  \"verified_complete\": true\n" +
		"}\n" +
		"```\n\n" +
		"### File 2: solution-design.json\n" +
		"CRITICAL: This MUST conform to the v2 schema. Key fields:\n" +
		"- demand_package_id: must match the id from demand-package.json\n" +
		"- wbs: MUST be an object with demand_package_id and specs array\n" +
		"- test_plan: object with demand_package_id, scenarios array; each scenario needs name (not feature), and given/when/then as arrays\n" +
		"- lane_assignments: array of {spec_id, lane, executor_type} -- executor_type must be speckit, stub, or direct\n" +
		"- eval_surface_contract: object with demand_package_id and surfaces array\n\n" +
		"Example solution-design.json structure:\n" +
		"```json\n" +
		"{\n" +
		"  \"id\": \"<same as demand_package_id>\",\n" +
		"  \"demand_package_id\": \"<run-id>\",\n" +
		"  \"test_plan\": {\n" +
		"    \"demand_package_id\": \"<run-id>\",\n" +
		"    \"scenarios\": [{\"id\": \"s1\", \"name\": \"...\", \"given\": [], \"when\": [], \"then\": [], \"kind\": \"e2e\"}]\n" +
		"  },\n" +
		"  \"wbs\": {\n" +
		"    \"demand_package_id\": \"<run-id>\",\n" +
		"    \"specs\": [{\"id\": \"spec-1\", \"description\": \"...\", \"prompt_path\": \".dft/specs/spec-1.md\", \"acceptance_criteria\": [\"...\"]}]\n" +
		"  },\n" +
		"  \"lane_assignments\": [\n" +
		"    {\"spec_id\": \"spec-1\", \"lane\": \"speckit\", \"executor_type\": \"speckit\"}\n" +
		"  ],\n" +
		"  \"eval_surface_contract\": {\n" +
		"    \"demand_package_id\": \"<run-id>\",\n" +
		"    \"surfaces\": [{\"id\": \"...\", \"kind\": \"topic\", \"artifact_ref\": \"...\", \"adapter_family\": \"copilot\", \"environment_class\": \"test\"}]\n" +
		"  }\n" +
		"}\n" +
		"```\n\n" +
		"## INSTRUCTIONS\n" +
		"1. Read the demand from README.md\n" +
		"2. Run the dft Intent phase: ambiguity scanner then demand refiner then AC verifier\n" +
		"3. Run the dft Solution phase: test planner then WBS author then lane assignment then surface contract author\n" +
		"4. Write demand-package.json and solution-design.json to " + designDir + "/\n" +
		"5. Use executor_type speckit for specs that need code generation\n" +
		"6. DO NOT ask questions -- make reasonable assumptions and produce the JSON files\n\n" +
		"Demand: " + demand

	args := []string{"-w", "chat", "-q", prompt, "-Q"}
	if h.Profile != "" {
		args = append(args, "--profile", h.Profile)
	}
	if h.Model != "" {
		args = append(args, "-m", h.Model)
	}

	cmd := exec.CommandContext(ctx, "hermes", args...)
	cmd.Env = append(os.Environ(), "HERMES_WORKTREE="+workspaceDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hermes design failed (exit code %v): %s", err, string(out))
	}
	return nil
}