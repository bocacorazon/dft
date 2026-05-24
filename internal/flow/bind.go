package flow

type ExecutionContext struct {
	Cwd string
	Env map[string]string
}

func BindInputs(definition Definition, inputs map[string]any) Definition {
	bound := definition
	result := &Result{
		Inputs:      cloneAnyMap(inputs),
		Vars:        map[string]string{},
		StepOutputs: map[string]map[string]any{},
	}
	bound.Steps = bindInputSteps(bound.Steps, result)
	bound.Stages = bindInputStages(bound.Stages, result)
	return bound
}

func BindDefinition(definition Definition, ctx ExecutionContext) Definition {
	bound := definition
	bound.Steps = bindSteps(bound.Steps, ctx)
	bound.Stages = bindStages(bound.Stages, ctx)
	return bound
}

func bindInputStages(stages []Stage, result *Result) []Stage {
	if len(stages) == 0 {
		return nil
	}
	bound := make([]Stage, len(stages))
	for i, stage := range stages {
		bound[i] = stage
		bound[i].ID = renderString(stage.ID, result)
		bound[i].Setup = bindInputSteps(stage.Setup, result)
		bound[i].Steps = bindInputSteps(stage.Steps, result)
		bound[i].After = bindInputSteps(stage.After, result)
	}
	return bound
}

func bindStages(stages []Stage, ctx ExecutionContext) []Stage {
	if len(stages) == 0 {
		return nil
	}
	bound := make([]Stage, len(stages))
	for i, stage := range stages {
		bound[i] = stage
		bound[i].Setup = bindSteps(stage.Setup, ctx)
		bound[i].Steps = bindSteps(stage.Steps, ctx)
		bound[i].After = bindSteps(stage.After, ctx)
	}
	return bound
}

func bindInputSteps(steps []Step, result *Result) []Step {
	if len(steps) == 0 {
		return nil
	}
	bound := make([]Step, len(steps))
	for i, step := range steps {
		bound[i] = renderStep(step, result)
		bound[i].Setup = bindInputSteps(step.Setup, result)
		bound[i].Steps = bindInputSteps(step.Steps, result)
	}
	return bound
}

func bindSteps(steps []Step, ctx ExecutionContext) []Step {
	if len(steps) == 0 {
		return nil
	}
	bound := make([]Step, len(steps))
	for i, step := range steps {
		bound[i] = step
		if appliesExecutionContext(step.Type) {
			if ctx.Cwd != "" {
				bound[i].Cwd = ctx.Cwd
			}
			bound[i].Env = mergeEnvs(step.Env, ctx.Env)
		}
		bound[i].Setup = bindSteps(step.Setup, ctx)
		bound[i].Steps = bindSteps(step.Steps, ctx)
	}
	return bound
}

func appliesExecutionContext(stepType StepType) bool {
	switch stepType {
	case StepCommand, StepAgent, StepTool, StepFunction:
		return true
	default:
		return false
	}
}

func mergeEnvs(base map[string]string, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	merged := make(map[string]string, len(base)+len(overlay))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overlay {
		merged[key] = value
	}
	return merged
}
