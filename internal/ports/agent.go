package ports

import "context"

// AgentRequest captures one auditable invocation of an agent.
type AgentRequest struct {
	AgentName string
	Prompt    string
	Demand    string
	RunID     string
}

// AgentResponse contains raw agent output before strict parsing.
type AgentResponse struct {
	Raw string
}

// AgentAdapter invokes an agent through a concrete runtime.
type AgentAdapter interface {
	Invoke(context.Context, AgentRequest) (AgentResponse, error)
}
