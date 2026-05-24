package ports

import "context"

// CommandRequest captures one auditable workflow command dispatch.
type CommandRequest struct {
	Command     string
	Input       string
	RunID       string
	Cwd         string
	Env         map[string]string
	Integration string
	Model       string
	AllowTools  bool
}

// CommandResponse contains captured command execution output.
type CommandResponse struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandDispatcher dispatches named workflow commands through a concrete runtime.
type CommandDispatcher interface {
	DispatchCommand(context.Context, CommandRequest) (CommandResponse, error)
}
