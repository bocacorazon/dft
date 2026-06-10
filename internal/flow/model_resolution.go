package flow

import (
	"fmt"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
)

// ResolveModels replaces model_type declarations with concrete model IDs.
func ResolveModels(definition Definition, config domain.ModelConfig, family string) (Definition, error) {
	resolved := definition
	var err error
	resolved.Steps, err = resolveStepModels(definition.Steps, config, family)
	if err != nil {
		return Definition{}, err
	}
	resolved.Stages, err = resolveStageModels(definition.Stages, config, family)
	if err != nil {
		return Definition{}, err
	}
	return resolved, nil
}

func resolveStageModels(stages []Stage, config domain.ModelConfig, family string) ([]Stage, error) {
	if len(stages) == 0 {
		return nil, nil
	}
	resolved := make([]Stage, len(stages))
	for i, stage := range stages {
		setup, err := resolveStepModels(stage.Setup, config, family)
		if err != nil {
			return nil, err
		}
		steps, err := resolveStepModels(stage.Steps, config, family)
		if err != nil {
			return nil, err
		}
		after, err := resolveStepModels(stage.After, config, family)
		if err != nil {
			return nil, err
		}
		resolved[i] = stage
		resolved[i].Setup = setup
		resolved[i].Steps = steps
		resolved[i].After = after
	}
	return resolved, nil
}

func resolveStepModels(steps []Step, config domain.ModelConfig, family string) ([]Step, error) {
	if len(steps) == 0 {
		return nil, nil
	}
	resolved := make([]Step, len(steps))
	for i, step := range steps {
		resolved[i] = step
		if strings.TrimSpace(step.ModelType) != "" {
			model, err := config.Resolve(family, domain.ModelTier(strings.TrimSpace(step.ModelType)))
			if err != nil {
				return nil, fmt.Errorf("step %q resolve model_type %q: %w", step.ID, step.ModelType, err)
			}
			resolved[i].Model = model
		}
		setup, err := resolveStepModels(step.Setup, config, family)
		if err != nil {
			return nil, err
		}
		nested, err := resolveStepModels(step.Steps, config, family)
		if err != nil {
			return nil, err
		}
		resolved[i].Setup = setup
		resolved[i].Steps = nested
	}
	return resolved, nil
}
