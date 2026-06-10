package ports

import "context"

// AgentRequest captures one auditable invocation of an agent.
type AgentRequest struct {
	AgentName  string
	Prompt     string
	Increment  string
	RunID      string
	StepID     string
	Attempt    int
	Cwd        string
	Env        map[string]string
	Model      string
	AllowTools bool
}

// AgentUsage captures best-effort observability for one agent invocation.
// Fields are zero when the underlying runtime does not expose them.
type AgentUsage struct {
	Model        string `json:"model,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	ExitCode     int    `json:"exit_code"`
}

// AgentResponse contains raw agent output before strict parsing.
type AgentResponse struct {
	Raw   string
	Usage AgentUsage
}

// AgentAdapter invokes an agent through a concrete runtime.
type AgentAdapter interface {
	Invoke(context.Context, AgentRequest) (AgentResponse, error)
}
