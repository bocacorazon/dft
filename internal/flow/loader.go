package flow

import (
	"fmt"
	"os"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
	"gopkg.in/yaml.v3"
)

type definitionDocument struct {
	SchemaVersion      string            `yaml:"schema_version"`
	MaxSpecParallelism int               `yaml:"max_spec_parallelism"`
	Workflow           workflowMetadata  `yaml:"workflow"`
	Inputs             map[string]any    `yaml:"inputs"`
	Steps              []definitionStep  `yaml:"steps"`
	Stages             []definitionStage `yaml:"stages"`
}

type workflowMetadata struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Author      string `yaml:"author"`
	Description string `yaml:"description"`
}

type definitionStage struct {
	ID     string           `yaml:"id"`
	Setup  []definitionStep `yaml:"setup"`
	Steps  []definitionStep `yaml:"steps"`
	After  []definitionStep `yaml:"after"`
	Verify []domain.Check   `yaml:"verify"`
}

type definitionStep struct {
	ID            string            `yaml:"id"`
	Type          StepType          `yaml:"type"`
	CommandName   string            `yaml:"command_name"`
	Command       yaml.Node         `yaml:"command"`
	CommandInput  string            `yaml:"command_input"`
	Input         definitionInput   `yaml:"input"`
	Integration   string            `yaml:"integration"`
	Model         string            `yaml:"model"`
	ModelType     string            `yaml:"model_type"`
	AgentName     string            `yaml:"agent_name"`
	OutputMode    AgentOutputMode   `yaml:"output_mode"`
	AllowTools    bool              `yaml:"allow_tools"`
	Prompt        string            `yaml:"prompt"`
	Increment     string            `yaml:"increment"`
	Cwd           string            `yaml:"cwd"`
	Env           map[string]string `yaml:"env"`
	Tool          []string          `yaml:"tool"`
	Function      string            `yaml:"function"`
	Args          map[string]string `yaml:"args"`
	MaxIterations int               `yaml:"max_iterations"`
	NoContext     bool              `yaml:"no_context"`
	Message       string            `yaml:"message"`
	Setup         []definitionStep  `yaml:"setup"`
	Verify        []domain.Check    `yaml:"verify"`
	Checks        []domain.Check    `yaml:"checks"`
	OnError       string            `yaml:"on_error"`
	Workflow      string            `yaml:"workflow"`
	Steps         []definitionStep  `yaml:"steps"`
	ExitWhen      map[string]string `yaml:"exit_when"`
	When          string            `yaml:"when"`
}

type definitionInput struct {
	Args string `yaml:"args"`
}

// ParseDefinition decodes a workflow definition from YAML content.
func ParseDefinition(content []byte) (Definition, error) {
	var document definitionDocument
	if err := yaml.Unmarshal(content, &document); err != nil {
		return Definition{}, fmt.Errorf("parse flow definition: %w", err)
	}
	definition, err := normalizeDefinition(document)
	if err != nil {
		return Definition{}, err
	}
	if err := validateDefinition(definition); err != nil {
		return Definition{}, err
	}
	return definition, nil
}

// LoadDefinition reads a minimal external flow definition from YAML.
func LoadDefinition(path string) (Definition, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("read flow definition: %w", err)
	}
	return ParseDefinition(content)
}

func normalizeDefinition(document definitionDocument) (Definition, error) {
	definition := Definition{
		MaxSpecParallelism: document.MaxSpecParallelism,
	}
	if len(document.Steps) > 0 {
		steps, err := normalizeSteps(document.Steps)
		if err != nil {
			return Definition{}, err
		}
		definition.Steps = steps
	}
	if len(document.Stages) > 0 {
		stages, err := normalizeStages(document.Stages)
		if err != nil {
			return Definition{}, err
		}
		definition.Stages = stages
	}
	return definition, nil
}

func normalizeStages(raw []definitionStage) ([]Stage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	stages := make([]Stage, len(raw))
	for i, stage := range raw {
		setup, err := normalizeSteps(stage.Setup)
		if err != nil {
			return nil, err
		}
		steps, err := normalizeSteps(stage.Steps)
		if err != nil {
			return nil, err
		}
		after, err := normalizeSteps(stage.After)
		if err != nil {
			return nil, err
		}
		stages[i] = Stage{
			ID:     stage.ID,
			Setup:  setup,
			Steps:  steps,
			After:  after,
			Verify: stage.Verify,
		}
	}
	return stages, nil
}

func normalizeSteps(raw []definitionStep) ([]Step, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	steps := make([]Step, len(raw))
	for i, step := range raw {
		normalized, err := normalizeStep(step)
		if err != nil {
			return nil, err
		}
		steps[i] = normalized
	}
	return steps, nil
}

func normalizeStep(step definitionStep) (Step, error) {
	commandName, toolCommand, err := decodeCommand(step.Command)
	if err != nil {
		return Step{}, err
	}
	if step.CommandName == "" {
		step.CommandName = commandName
	}
	if len(step.Tool) == 0 {
		step.Tool = toolCommand
	}
	stepType := inferStepType(step)
	if stepType == "" {
		return Step{}, fmt.Errorf("step %q type or executable action is required", step.ID)
	}
	setup, err := normalizeSteps(step.Setup)
	if err != nil {
		return Step{}, err
	}
	nested, err := normalizeSteps(step.Steps)
	if err != nil {
		return Step{}, err
	}
	commandInput := step.CommandInput
	if commandInput == "" {
		commandInput = step.Input.Args
	}
	return Step{
		ID:            step.ID,
		Type:          stepType,
		CommandName:   step.CommandName,
		CommandInput:  commandInput,
		Integration:   step.Integration,
		Model:         step.Model,
		ModelType:     step.ModelType,
		AgentName:     step.AgentName,
		OutputMode:    step.OutputMode,
		AllowTools:    step.AllowTools,
		Prompt:        step.Prompt,
		Increment:     step.Increment,
		Cwd:           step.Cwd,
		Env:           step.Env,
		Command:       step.Tool,
		Function:      step.Function,
		Args:          step.Args,
		MaxIterations: step.MaxIterations,
		NoContext:     step.NoContext,
		Message:       step.Message,
		Setup:         setup,
		Verify:        step.Verify,
		Checks:        step.Checks,
		OnError:       step.OnError,
		Workflow:      step.Workflow,
		Steps:         nested,
		ExitWhen:      step.ExitWhen,
		When:          step.When,
	}, nil
}

func decodeCommand(node yaml.Node) (string, []string, error) {
	if node.Kind == 0 {
		return "", nil, nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return "", nil, fmt.Errorf("decode command step: %w", err)
		}
		return value, nil, nil
	case yaml.SequenceNode:
		var value []string
		if err := node.Decode(&value); err != nil {
			return "", nil, fmt.Errorf("decode tool command step: %w", err)
		}
		return "", value, nil
	default:
		return "", nil, fmt.Errorf("command must be a string or string list")
	}
}

func inferStepType(step definitionStep) StepType {
	if step.Type != "" {
		return step.Type
	}
	switch {
	case step.CommandName != "":
		return StepCommand
	case len(step.Tool) > 0:
		return StepTool
	case step.AgentName != "":
		return StepAgent
	case step.Function != "":
		return StepFunction
	case step.Workflow != "":
		return StepWorkflow
	case step.Message != "":
		return StepGate
	default:
		return ""
	}
}

func validateDefinition(definition Definition) error {
	if definition.MaxSpecParallelism < 0 {
		return fmt.Errorf("max_spec_parallelism cannot be negative")
	}
	if err := validateSteps(definition.Steps); err != nil {
		return err
	}
	for _, stage := range definition.Stages {
		if err := validateSteps(stage.Setup); err != nil {
			return err
		}
		if err := validateSteps(stage.Steps); err != nil {
			return err
		}
		if err := validateSteps(stage.After); err != nil {
			return err
		}
	}
	return nil
}

func validateSteps(steps []Step) error {
	for _, step := range steps {
		if step.MaxIterations < 0 {
			return fmt.Errorf("step %q max_iterations cannot be negative", step.ID)
		}
		if err := validateStepShape(step); err != nil {
			return err
		}
		if err := validateSteps(step.Setup); err != nil {
			return err
		}
		if err := validateSteps(step.Steps); err != nil {
			return err
		}
	}
	return nil
}

// validateStepShape enforces that a step carries exactly one executable shape
// (command/agent/gate/tool/function/workflow/verify/loop) and that the shape
// matching its resolved type has the payload it needs. `verify:`/`checks:`
// blocks attached to non-verify steps are post-conditions, not a second shape.
func validateStepShape(step Step) error {
	var shapes []string
	if step.CommandName != "" {
		shapes = append(shapes, "command")
	}
	if len(step.Command) > 0 {
		shapes = append(shapes, "tool")
	}
	if step.AgentName != "" {
		shapes = append(shapes, "agent")
	}
	if step.Function != "" {
		shapes = append(shapes, "function")
	}
	if step.Workflow != "" {
		shapes = append(shapes, "workflow")
	}
	if step.Message != "" {
		shapes = append(shapes, "gate")
	}
	if step.Type == StepVerify && (len(step.Verify) > 0 || len(step.Checks) > 0) {
		shapes = append(shapes, "verify")
	}
	if step.Type == StepLoop {
		shapes = append(shapes, "loop")
	}
	if len(shapes) > 1 {
		return fmt.Errorf("step %q has conflicting shapes (%s); exactly one of command/agent/gate/tool/function/workflow/verify/loop is allowed", step.ID, strings.Join(shapes, ", "))
	}

	switch step.Type {
	case StepCommand:
		if step.CommandName == "" {
			return fmt.Errorf("command step %q requires command_name", step.ID)
		}
	case StepTool:
		if len(step.Command) == 0 {
			return fmt.Errorf("tool step %q requires command", step.ID)
		}
	case StepAgent:
		if step.AgentName == "" {
			return fmt.Errorf("agent step %q requires agent_name", step.ID)
		}
	case StepFunction:
		if step.Function == "" {
			return fmt.Errorf("function step %q requires function", step.ID)
		}
	case StepGate:
		if step.Message == "" {
			return fmt.Errorf("gate step %q requires message", step.ID)
		}
	case StepWorkflow:
		if step.Workflow == "" && step.Args["path"] == "" {
			return fmt.Errorf("workflow step %q requires a workflow path", step.ID)
		}
	case StepVerify:
		if len(step.Verify) == 0 && len(step.Checks) == 0 {
			return fmt.Errorf("verify step %q requires checks", step.ID)
		}
	case StepLoop:
		if step.MaxIterations <= 0 {
			return fmt.Errorf("loop step %q requires max_iterations", step.ID)
		}
		if len(step.Steps) == 0 {
			return fmt.Errorf("loop step %q requires steps", step.ID)
		}
	}
	return nil
}
