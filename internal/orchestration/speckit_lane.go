package orchestration

import (
	"path/filepath"

	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/flow"
)

// BuildSpecKitLane creates the v1 spec lane steps for one spec worktree.
func BuildSpecKitLane(spec domain.SpecRef, worktree SpecWorktree) flow.Definition {
	env := map[string]string{
		"SPECIFY_FEATURE_DIRECTORY": filepath.Join("specs", spec.ID),
	}
	for key, value := range worktree.SpecKitEnv {
		env[key] = value
	}

	steps := []flow.Step{
		{
			ID:        "specify",
			Type:      flow.StepAgent,
			AgentName: "speckit.specify.agent.md",
			Prompt:    spec.Description,
			Demand:    spec.Description,
			Cwd:       worktree.WorktreePath,
			Env:       cloneEnv(env),
		},
		{
			ID:        "plan",
			Type:      flow.StepAgent,
			AgentName: "speckit.plan.agent.md",
			Prompt:    spec.Description,
			Demand:    spec.Description,
			Cwd:       worktree.WorktreePath,
			Env:       cloneEnv(env),
		},
		{
			ID:        "tasks",
			Type:      flow.StepAgent,
			AgentName: "speckit.tasks.agent.md",
			Prompt:    spec.Description,
			Demand:    spec.Description,
			Cwd:       worktree.WorktreePath,
			Env:       cloneEnv(env),
		},
		{
			ID:        "implement",
			Type:      flow.StepAgent,
			AgentName: "speckit.implement.agent.md",
			Prompt:    spec.Description,
			Demand:    spec.Description,
			Cwd:       worktree.WorktreePath,
			Env:       cloneEnv(env),
		},
	}
	return flow.Definition{Steps: steps}
}

func cloneEnv(env map[string]string) map[string]string {
	clone := make(map[string]string, len(env))
	for key, value := range env {
		clone[key] = value
	}
	return clone
}
