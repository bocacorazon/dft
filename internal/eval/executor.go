package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

const (
	ActionCLIRun                  = "cli.run"
	ActionCLIExpectExitCode       = "cli.expect_exit_code"
	ActionCLIExpectStdoutContains = "cli.expect_stdout_contains"
	ActionCLIExpectStderrContains = "cli.expect_stderr_contains"
	ActionFileExists              = "file.exists"
	ActionFileContains            = "file.contains"
	ActionHTTPGet                 = "http.get"
	ActionHTTPPostJSON            = "http.post_json"
	ActionHTTPExpectStatus        = "http.expect_status"
	ActionHTTPExpectJSONPath      = "http.expect_json_path_equals"
)

// Executor runs artifact-bound BDD packs and optional legacy deterministic checks.
type Executor struct {
	RootDir    string
	HTTPClient *http.Client
	Verifier   ports.Verifier
}

// Execute runs a validated eval plan and writes `.dft/runs/<run-id>/eval/evaluation.json` when runID is non-empty.
func (e Executor) Execute(ctx context.Context, runID string, ready domain.EvalReady, plan domain.EvalPlan) (domain.EvalResult, error) {
	if err := plan.Validate(); err != nil {
		return domain.EvalResult{}, fmt.Errorf("validate eval plan: %w", err)
	}
	result := domain.EvalResult{
		Status:    domain.EvalStatusPass,
		Readiness: &ready,
	}
	if ready.Status == domain.EvalStatusBlocked {
		result.Status = domain.EvalStatusBlocked
		result.Findings = append(result.Findings, ready.Findings...)
		result.Coverage = coverageFor(plan, nil)
		if err := e.writeResult(runID, result); err != nil {
			return domain.EvalResult{}, err
		}
		return result, nil
	}

	bindings := map[string]domain.SurfaceBinding{}
	for _, binding := range ready.Bindings {
		bindings[binding.SurfaceID] = binding
	}
	evidenceDir := ""
	if runID != "" {
		evidenceDir = filepath.Join(e.root(), ".dft", "runs", runID, "eval", "evidence")
	}

	passedRequirements := map[string]struct{}{}
	for _, pack := range plan.Packs {
		execution := e.executePack(ctx, pack, bindings, evidenceDir)
		result.Executions = append(result.Executions, execution)
		if execution.Status != domain.EvalStatusPass {
			if execution.Status == domain.EvalStatusBlocked {
				result.Status = domain.EvalStatusBlocked
			} else if result.Status != domain.EvalStatusBlocked {
				result.Status = domain.EvalStatusFail
			}
			result.Findings = append(result.Findings, execution.Findings...)
			continue
		}
		for _, scenario := range execution.Scenarios {
			if scenario.Status != domain.EvalStatusPass {
				continue
			}
			for _, requirementID := range scenario.RequirementIDs {
				passedRequirements[requirementID] = struct{}{}
			}
		}
	}

	if len(plan.Checks) > 0 {
		if e.Verifier == nil {
			return domain.EvalResult{}, fmt.Errorf("verifier is required for deterministic eval checks")
		}
		checkResult := e.Verifier.Run(ctx, plan.Checks)
		result.Checks = checkResult.Results
		if checkResult.Status != domain.VerdictPass {
			result.Status = domain.EvalStatusFail
			result.Findings = append(result.Findings, checkResult.Findings...)
		}
	}

	result.Coverage = coverageFor(plan, passedRequirements)
	if result.Coverage.Total > 0 && result.Coverage.Covered < result.Coverage.Total && result.Status == domain.EvalStatusPass {
		result.Status = domain.EvalStatusFail
		for _, requirementID := range result.Coverage.Uncovered {
			result.Findings = append(result.Findings, domain.Finding{
				CheckID:  "coverage-" + requirementID,
				Severity: "high",
				Category: "eval-coverage",
				Message:  "requirement has no passing eval scenario",
				Location: requirementID,
			})
		}
	}
	if err := e.writeResult(runID, result); err != nil {
		return domain.EvalResult{}, err
	}
	return result, nil
}

func (e Executor) executePack(ctx context.Context, pack domain.BDDPack, bindings map[string]domain.SurfaceBinding, evidenceDir string) domain.PackExecution {
	execution := domain.PackExecution{
		PackID:    pack.ID,
		SurfaceID: pack.SurfaceID,
		Status:    domain.EvalStatusPass,
	}
	binding, ok := bindings[pack.SurfaceID]
	if !ok {
		execution.Status = domain.EvalStatusBlocked
		execution.Findings = append(execution.Findings, domain.Finding{
			CheckID:  "pack-binding-" + pack.ID,
			Severity: "high",
			Category: "eval-readiness",
			Message:  fmt.Sprintf("BDD pack %q references unbound surface %q", pack.ID, pack.SurfaceID),
		})
		return execution
	}
	for _, scenario := range pack.Scenarios {
		outcome := e.executeScenario(ctx, binding, scenario, evidenceDir)
		execution.Scenarios = append(execution.Scenarios, outcome)
		if outcome.Status != domain.EvalStatusPass {
			execution.Status = outcome.Status
			if execution.Status == domain.EvalStatusBlocked {
				execution.Status = domain.EvalStatusFail
			}
			execution.Findings = append(execution.Findings, domain.Finding{
				CheckID:  scenario.ID,
				Severity: "high",
				Category: "eval-bdd",
				Message:  outcome.Message,
				Location: pack.SurfaceID,
			})
		}
	}
	return execution
}

func (e Executor) executeScenario(ctx context.Context, binding domain.SurfaceBinding, scenario domain.BDDScenario, evidenceDir string) domain.ScenarioOutcome {
	state := scenarioState{}
	outcome := domain.ScenarioOutcome{
		ScenarioID:     scenario.ID,
		RequirementIDs: append([]string(nil), scenario.RequirementIDs...),
		Status:         domain.EvalStatusPass,
	}
	for _, step := range scenario.Steps {
		if err := e.executeStep(ctx, binding, &state, step); err != nil {
			outcome.Status = domain.EvalStatusFail
			outcome.Message = err.Error()
			return outcome
		}
	}
	if evidenceDir != "" {
		if err := e.writeScenarioEvidence(evidenceDir, scenario.ID, &state); err != nil {
			outcome.Status = domain.EvalStatusError
			outcome.Message = err.Error()
			return outcome
		}
	}
	if state.stdoutPath != "" {
		outcome.Evidence = append(outcome.Evidence, domain.Evidence{ID: scenario.ID + "-stdout", Kind: "stdout", Path: state.stdoutPath})
	}
	if state.stderrPath != "" {
		outcome.Evidence = append(outcome.Evidence, domain.Evidence{ID: scenario.ID + "-stderr", Kind: "stderr", Path: state.stderrPath})
	}
	if state.bodyPath != "" {
		outcome.Evidence = append(outcome.Evidence, domain.Evidence{ID: scenario.ID + "-http-body", Kind: "http_body", Path: state.bodyPath})
	}
	return outcome
}

type scenarioState struct {
	exitCode   int
	stdout     string
	stderr     string
	stdoutPath string
	stderrPath string
	bodyPath   string
	statusCode int
	body       []byte
}

func (e Executor) writeScenarioEvidence(evidenceDir string, scenarioID string, state *scenarioState) error {
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return fmt.Errorf("create eval evidence directory: %w", err)
	}
	if state.stdout != "" {
		path := filepath.Join(evidenceDir, scenarioID+"-stdout.txt")
		if err := os.WriteFile(path, []byte(state.stdout), 0o644); err != nil {
			return fmt.Errorf("write stdout evidence: %w", err)
		}
		state.stdoutPath = e.rel(path)
	}
	if state.stderr != "" {
		path := filepath.Join(evidenceDir, scenarioID+"-stderr.txt")
		if err := os.WriteFile(path, []byte(state.stderr), 0o644); err != nil {
			return fmt.Errorf("write stderr evidence: %w", err)
		}
		state.stderrPath = e.rel(path)
	}
	if len(state.body) > 0 {
		path := filepath.Join(evidenceDir, scenarioID+"-http-body.txt")
		if err := os.WriteFile(path, state.body, 0o644); err != nil {
			return fmt.Errorf("write HTTP body evidence: %w", err)
		}
		state.bodyPath = e.rel(path)
	}
	return nil
}

func (e Executor) executeStep(ctx context.Context, binding domain.SurfaceBinding, state *scenarioState, step domain.BDDStep) error {
	switch step.Action {
	case ActionCLIRun:
		artifactPath := binding.Path
		if artifactPath == "" {
			artifactPath = binding.URI
		}
		if artifactPath == "" {
			return fmt.Errorf("cli.run requires bound artifact path or uri")
		}
		argv := append([]string{e.path(artifactPath)}, step.Args...)
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = e.root()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		state.stdout = stdout.String()
		state.stderr = stderr.String()
		state.exitCode = 0
		if err != nil {
			state.exitCode = 1
			if exitErr, ok := err.(*exec.ExitError); ok {
				state.exitCode = exitErr.ExitCode()
			}
		}
		return nil
	case ActionCLIExpectExitCode:
		if len(step.Args) != 1 {
			return fmt.Errorf("cli.expect_exit_code requires expected code")
		}
		expected, err := strconv.Atoi(step.Args[0])
		if err != nil {
			return fmt.Errorf("cli.expect_exit_code expected code must be integer: %q", step.Args[0])
		}
		if state.exitCode != expected {
			return fmt.Errorf("exit code = %d, want %d", state.exitCode, expected)
		}
		return nil
	case ActionCLIExpectStdoutContains:
		if len(step.Args) != 1 {
			return fmt.Errorf("cli.expect_stdout_contains requires expected substring")
		}
		if !strings.Contains(state.stdout, step.Args[0]) {
			return fmt.Errorf("stdout does not contain %q", step.Args[0])
		}
		return nil
	case ActionCLIExpectStderrContains:
		if len(step.Args) != 1 {
			return fmt.Errorf("cli.expect_stderr_contains requires expected substring")
		}
		if !strings.Contains(state.stderr, step.Args[0]) {
			return fmt.Errorf("stderr does not contain %q", step.Args[0])
		}
		return nil
	case ActionFileExists:
		if len(step.Args) != 1 {
			return fmt.Errorf("file.exists requires path")
		}
		if _, err := os.Stat(e.path(step.Args[0])); err != nil {
			return fmt.Errorf("expected file %s to exist: %v", step.Args[0], err)
		}
		return nil
	case ActionFileContains:
		if len(step.Args) != 2 {
			return fmt.Errorf("file.contains requires path and substring")
		}
		content, err := os.ReadFile(e.path(step.Args[0]))
		if err != nil {
			return fmt.Errorf("read %s: %v", step.Args[0], err)
		}
		if !strings.Contains(string(content), step.Args[1]) {
			return fmt.Errorf("%s does not contain %q", step.Args[0], step.Args[1])
		}
		return nil
	case ActionHTTPGet:
		if len(step.Args) != 1 {
			return fmt.Errorf("http.get requires path or URL")
		}
		return e.doHTTP(ctx, binding, state, http.MethodGet, step.Args[0], "")
	case ActionHTTPPostJSON:
		if len(step.Args) != 2 {
			return fmt.Errorf("http.post_json requires path or URL and JSON body")
		}
		return e.doHTTP(ctx, binding, state, http.MethodPost, step.Args[0], step.Args[1])
	case ActionHTTPExpectStatus:
		if len(step.Args) != 1 {
			return fmt.Errorf("http.expect_status requires expected status")
		}
		expected, err := strconv.Atoi(step.Args[0])
		if err != nil {
			return fmt.Errorf("http.expect_status expected status must be integer: %q", step.Args[0])
		}
		if state.statusCode != expected {
			return fmt.Errorf("HTTP status = %d, want %d", state.statusCode, expected)
		}
		return nil
	case ActionHTTPExpectJSONPath:
		if len(step.Args) != 2 {
			return fmt.Errorf("http.expect_json_path_equals requires JSON path and expected value")
		}
		var document any
		if err := json.Unmarshal(state.body, &document); err != nil {
			return fmt.Errorf("parse HTTP response JSON: %w", err)
		}
		value, ok := lookupJSONPath(document, step.Args[0])
		if !ok {
			return fmt.Errorf("JSON path %s not found", step.Args[0])
		}
		if got := fmt.Sprint(value); got != step.Args[1] {
			return fmt.Errorf("JSON path %s = %q, want %q", step.Args[0], got, step.Args[1])
		}
		return nil
	default:
		return fmt.Errorf("unsupported BDD action %q", step.Action)
	}
}

func (e Executor) doHTTP(ctx context.Context, binding domain.SurfaceBinding, state *scenarioState, method string, pathOrURL string, body string) error {
	url := pathOrURL
	if strings.HasPrefix(pathOrURL, "/") {
		if binding.URI == "" {
			return fmt.Errorf("%s requires bound artifact URI for relative HTTP path", method)
		}
		url = strings.TrimRight(binding.URI, "/") + pathOrURL
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return fmt.Errorf("build HTTP request: %w", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := e.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read HTTP response: %w", err)
	}
	state.statusCode = resp.StatusCode
	state.body = content
	return nil
}

func coverageFor(plan domain.EvalPlan, passed map[string]struct{}) domain.RequirementCoverage {
	requirements := append([]string(nil), plan.RequirementIDs...)
	if len(requirements) == 0 {
		seen := map[string]struct{}{}
		for _, pack := range plan.Packs {
			for _, scenario := range pack.Scenarios {
				for _, requirementID := range scenario.RequirementIDs {
					if _, ok := seen[requirementID]; ok {
						continue
					}
					seen[requirementID] = struct{}{}
					requirements = append(requirements, requirementID)
				}
			}
		}
	}
	coverage := domain.RequirementCoverage{Total: len(requirements)}
	for _, requirementID := range requirements {
		if _, ok := passed[requirementID]; ok {
			coverage.Covered++
			continue
		}
		coverage.Uncovered = append(coverage.Uncovered, requirementID)
	}
	return coverage
}

func (e Executor) writeResult(runID string, result domain.EvalResult) error {
	if runID == "" {
		return nil
	}
	return writeJSON(filepath.Join(e.root(), ".dft", "runs", runID, "eval", "evaluation.json"), result)
}

func (e Executor) path(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(e.root(), path)
}

func (e Executor) rel(path string) string {
	relative, err := filepath.Rel(e.root(), path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func (e Executor) root() string {
	if e.RootDir == "" {
		return "."
	}
	return e.RootDir
}

func lookupJSONPath(document any, path string) (any, bool) {
	if path == "" || path == "$" {
		return document, true
	}
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, ".")
	current := document
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
