package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func (h *Harness) Run(ctx context.Context, increment string) (*RunResult, error) {
	start := time.Now()
	runID := time.Now().Format("run-20060102-150405")

	// Phase 1: Provision Bare Remote & Clone
	remoteDir, workspaceDir, err := h.provisionRepos(runID, increment)
	if err != nil {
		return nil, err
	}

	result := &RunResult{
		RunID:        runID,
		Increment:    increment,
		WorkspaceDir: workspaceDir,
		RemoteDir:    remoteDir,
	}

	// Save increment for retry loops
	h.lastIncrement = increment

	// Phase 2: Design
	result.DesignDir = filepath.Join(workspaceDir, ".dft", "runs", runID, "design")
	if err := h.Agent.Design(ctx, increment, workspaceDir, result.DesignDir); err != nil {
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

func (h *Harness) runBuild(runID, repoDir string) (string, bool) {
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
