# dft Evaluation Harness Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Create a reusable Go package (`internal/eval/harness`) that runs a full "demand → design → build → evaluate" cycle. It provisions a real Git repository with a remote, uses a real LLM agent (Hermes) for design, executes `dft build` with a real adapter, and validates the result exclusively by parsing the generated `evaluation.json`.

**Architecture:**
```text
┌─────────────────────────────────────────────────────────┐
│ Harness.Run(demand, opts)                               │
│                                                         │
│  1. Setup Bare Remote & Workspace Clone                 │
│  2. Write demand to README.md + Push                    │
│  3. HermesAgent: Zero-shot Intent & Solution phases     │
│  4. RunBuild: `dft build <id> --adapter copilot`        │
│  5. RunEval: `dft evaluate <id>`                        │
│  6. Parse `evaluation.json` strictly for verdict        │
│  7. Return RunResult                                    │
└─────────────────────────────────────────────────────────┘
```

**Key design decisions based on constraints:**
1. **JSON parsing only:** No fragile stdout `strings.Contains` for evaluation results. The harness strictly unmarshals `.dft/runs/<id>/eval/evaluation.json` to determine pass/fail.
2. **Real executors only:** The `stub` adapter is banned. `dft build` runs with a real coding agent adapter (e.g., `copilot`) so real software is produced.
3. **HermesAgent Integration:** The design phase executes `hermes chat -q` with a zero-shot prompt, forcing the agent to run the Intent and Solution skills automatically without user interaction.
4. **Real Remote Repositories:** Testing branch/mergeback logic requires a real origin. The harness creates a local bare repository (`repo.git`), clones it (`workspace`), writes the prompt to `README.md`, provisions `dft`, and pushes to `origin` before starting the pipeline.

---

## Task 1: Define Core Types and Interfaces

**Objective:** Set up the package with all type definitions and the main orchestrator interface.

**Files:**
- Create: `internal/eval/harness/interfaces.go`
- Create: `internal/eval/harness/harness.go`

**interfaces.go:**

```go
package harness

import (
	"context"
	"time"
)

// Agent produces a solution-design.json from raw demand text.
type Agent interface {
	Design(ctx context.Context, demand string, workspaceDir string, designDir string) error
	Name() string
}

// RunResult captures the complete output of one pipeline execution.
type RunResult struct {
	RunID       string        
	Demand      string        
	DesignDir   string        
	DesignError string        
	BuildOK     bool          
	BuildOutput string        
	EvalVerdict string        // parsed from JSON: "pass", "blocked", "error"
	EvalOutput  string        
	WorkspaceDir string       // Path to cloned test repo
	RemoteDir    string       // Path to bare remote repo
	TotalTime   time.Duration 
}

// Harness is the top-level orchestrator.
type Harness struct {
	DftPath string
	Agent   Agent
	Adapter string // e.g. "copilot"
}

func New(dftPath string, agent Agent, adapter string) *Harness {
	if adapter == "stub" {
		panic("stub adapter is disabled for evaluation harness")
	}
	return &Harness{
		DftPath: dftPath,
		Agent:   agent,
		Adapter: adapter,
	}
}
```

**harness.go:**

```go
package harness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (h *Harness) Run(ctx context.Context, demand string) (*RunResult, error) {
	start := time.Now()
	runID := time.Now().Format("run-20060102-150405")
	
	// Phase 1: Provision Bare Remote & Clone
	remoteDir, workspaceDir, err := h.provisionRepos(runID, demand)
	if err != nil {
		return nil, err
	}

	result := &RunResult{
		RunID:        runID,
		Demand:       demand,
		WorkspaceDir: workspaceDir,
		RemoteDir:    remoteDir,
	}

	// Phase 2: Design
	result.DesignDir = filepath.Join(workspaceDir, ".dft", "runs", runID, "design")
	if err := h.Agent.Design(ctx, demand, workspaceDir, result.DesignDir); err != nil {
		result.DesignError = err.Error()
		result.TotalTime = time.Since(start)
		return result, nil
	}

	// Phase 3: Build
	result.BuildOutput, result.BuildOK = h.runBuild(runID, workspaceDir)

	// Phase 4: Evaluate
	result.EvalOutput, result.EvalVerdict = h.runEval(runID, workspaceDir)

	result.TotalTime = time.Since(start)
	return result, nil
}
```

---

## Task 2: Implement Real Repository Provisioning

**Objective:** Create a bare remote, clone it, write the demand to `README.md`, provision `dft`, and push.

**Files:**
- Create: `internal/eval/harness/repo.go`

```go
package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// provisionRepos creates a bare remote and a cloned workspace.
func (h *Harness) provisionRepos(runID, demand string) (remoteDir string, workspaceDir string, err error) {
	baseTemp, err := os.MkdirTemp("", "dft-eval-"+runID+"-*")
	if err != nil {
		return "", "", err
	}

	remoteDir = filepath.Join(baseTemp, "remote.git")
	workspaceDir = filepath.Join(baseTemp, "workspace")

	// 1. Create bare remote
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		return "", "", err
	}
	if err := runIn(remoteDir, "git", "init", "--bare", "-b", "main"); err != nil {
		return "", "", err
	}

	// 2. Clone to workspace
	if err := runIn(baseTemp, "git", "clone", remoteDir, "workspace"); err != nil {
		return "", "", err
	}

	// Config Git
	runIn(workspaceDir, "git", "config", "user.name", "dft-eval")
	runIn(workspaceDir, "git", "config", "user.email", "eval@dft.local")

	// 3. Write README.md with demand
	readme := filepath.Join(workspaceDir, "README.md")
	content := fmt.Sprintf("# Evaluation Target\n\n**Demand:**\n%s\n", demand)
	if err := os.WriteFile(readme, []byte(content), 0644); err != nil {
		return "", "", err
	}
	
	// 4. Provision dft
	if err := runIn(workspaceDir, h.DftPath, "init"); err != nil {
		return "", "", err
	}

	// 5. Commit and Push
	runIn(workspaceDir, "git", "add", ".")
	runIn(workspaceDir, "git", "commit", "-m", "initial: provision repo and demand")
	if err := runIn(workspaceDir, "git", "push", "origin", "main"); err != nil {
		return "", "", err
	}

	return remoteDir, workspaceDir, nil
}

func runIn(dir, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func runInWithOutput(dir, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
```

---

## Task 3: Implement `HermesAgent`

**Objective:** Define the real Hermes agent adapter that calls `hermes chat -q` and forces the Intent and Solution phases without user interactivity.

**Files:**
- Create: `internal/eval/harness/hermes_agent.go`

```go
package harness

import (
	"context"
	"fmt"
	"os/exec"
)

type HermesAgent struct {
	Profile string // Optional hermes profile (e.g. "dft-eval")
	Model   string // Optional model override
}

func (h *HermesAgent) Name() string { return "hermes" }

func (h *HermesAgent) Design(ctx context.Context, demand string, workspaceDir string, designDir string) error {
	prompt := fmt.Sprintf(`Assume the role of the architecture team.
You must execute the dft Intent and Solution phases for the demand described in README.md.

Instructions:
1. Load the 'dft-intent' and 'dft-solution' skills (or use the individual agents if separated: ambiguity-scanner, demand-refiner, test-planner, wbs-author, etc).
2. DO NOT ask clarifying questions. Make reasonable assumptions for any ambiguities.
3. Generate the demand-package.json and solution-design.json.
4. Save them exactly to the '%s' directory.
5. Terminate when the JSON files have been successfully written to disk.

Demand text reference:
%s`, designDir, demand)

	args := []string{"chat", "-q", prompt}
	if h.Profile != "" {
		args = append(args, "--profile", h.Profile)
	}
	if h.Model != "" {
		args = append(args, "-m", h.Model)
	}

	cmd := exec.CommandContext(ctx, "hermes", args...)
	cmd.Dir = workspaceDir
	
	// In a real harness, you might want to capture this to a log file
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hermes design failed (exit code %v): %s", err, string(out))
	}
	return nil
}
```

---

## Task 4: Implement Build and Parse-Strict Evaluation

**Objective:** Run the actual executors, and strictly parse `evaluation.json` for validation.

**Files:**
- Modify: `internal/eval/harness/harness.go`

```go
// ... appending to harness.go

func (h *Harness) runBuild(runID, repoDir string) (string, bool) {
	// Build strictly prevents stub. Uses the configured adapter.
	out, err := runInWithOutput(repoDir, h.DftPath, "build", runID, "--adapter", h.Adapter)
	return out, err == nil
}

type EvalJSON struct {
	Verdict string `json:"verdict"`
	// add coverage, checks, findings here if needed for deeper inspect
}

func (h *Harness) runEval(runID, repoDir string) (string, string) {
	out, _ := runInWithOutput(repoDir, h.DftPath, "evaluate", runID)
	
	evalFile := filepath.Join(repoDir, ".dft", "runs", runID, "eval", "evaluation.json")
	data, err := os.ReadFile(evalFile)
	if err != nil {
		return out, "error: file not found"
	}

	var evalData EvalJSON
	if err := json.Unmarshal(data, &evalData); err != nil {
		return out, "error: invalid json"
	}

	if evalData.Verdict == "" {
		return out, "error: missing verdict"
	}

	return out, evalData.Verdict
}
```

---

## Task 5: Integration Tests

**Objective:** Write tests. Because `HermesAgent` + `copilot` is slow and costs money, we still keep a `SimulatedAgent` just for the unit test of this harness package itself, but ensure `stub` is bypassed. 

**Files:**
- Create: `internal/eval/harness/harness_test.go`
*(Test covers the mechanics using a mock agent. It will verify that JSON parsing catches errors, and the remote mapping works.)*

---

## Final Review of Changes
1. **JSON Parsing (#1):** `runEval` now exclusively unmarshals `evaluation.json`.
2. **No Stub (#2):** `New()` panics if initialized with `"stub"`. 
3. **HermesAgent (#3):** The real `hermes chat -q` agent adapter is written, executing zero-shot design from the `README.md`.
4. **Real Repo (#4):** `provisionRepos` creates `remote.git` (bare) and `workspace` (clone), writes the demand, commits, and pushes to `origin` before anything starts.
