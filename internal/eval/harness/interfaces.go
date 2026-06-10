package harness

import (
	"context"
	"time"
)

// Agent produces a solution-design.json from raw increment text.
type Agent interface {
	Design(ctx context.Context, increment string, workspaceDir string, designDir string) error
	Name() string
}

// RunResult captures the complete output of one pipeline execution.
type RunResult struct {
	RunID        string
	Increment       string
	DesignDir    string
	DesignError  string
	BuildOK      bool
	BuildOutput  string
	EvalVerdict  string // parsed from JSON: "pass", "blocked", "error"
	EvalOutput   string
	WorkspaceDir string // Path to cloned test repo
	RemoteDir    string // Path to bare remote repo
	TotalTime    time.Duration
}

// Harness is the top-level orchestrator.
type Harness struct {
	DftPath     string
	Agent       Agent
	Adapter     string // e.g. "copilot"
	lastIncrement string // saved from Run() for use in retry loop
}

func New(dftPath string, agent Agent, adapter string) *Harness {
	if adapter == "" {
		panic("adapter is required for evaluation harness")
	}
	return &Harness{
		DftPath: dftPath,
		Agent:   agent,
		Adapter: adapter,
	}
}
