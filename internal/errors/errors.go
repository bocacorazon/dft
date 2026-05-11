package errors

import "fmt"

type InvalidFlowError struct {
	Message string
}

func (e InvalidFlowError) Error() string {
	return fmt.Sprintf("invalid flow: %s", e.Message)
}

type UnsupportedStepTypeError struct {
	StepID   string
	StepType string
}

func (e UnsupportedStepTypeError) Error() string {
	if e.StepID == "" {
		return fmt.Sprintf("unsupported step type: %s", e.StepType)
	}
	return fmt.Sprintf("unsupported step type %q for step %q", e.StepType, e.StepID)
}

type RunNotFoundError struct {
	RunID string
}

func (e RunNotFoundError) Error() string {
	return fmt.Sprintf("run not found: %s", e.RunID)
}

type MissingExportError struct {
	StepID string
	Name   string
}

func (e MissingExportError) Error() string {
	return fmt.Sprintf("missing export %q required by step %q", e.Name, e.StepID)
}
