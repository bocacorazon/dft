package flow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/bocacorazon/dft/internal/ports"
)

// AgentCallRecord is one durable observability entry for a single agent
// invocation (one attempt). Records are appended to
// .dft/runs/<run-id>/agent-calls.jsonl as newline-delimited JSON.
type AgentCallRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	RunID        string    `json:"run_id"`
	StepID       string    `json:"step_id"`
	AgentName    string    `json:"agent_name"`
	Model        string    `json:"model,omitempty"`
	Attempt      int       `json:"attempt"`
	DurationMs   int64     `json:"duration_ms"`
	ExitCode     int       `json:"exit_code"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// invokeAgentObserved invokes the agent, measures wall-clock duration, and
// appends a durable observability record for the attempt before returning.
func (r Runner) invokeAgentObserved(ctx context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	start := time.Now()
	response, err := r.Agent.Invoke(ctx, request)
	wallMs := time.Since(start).Milliseconds()
	r.appendAgentCall(request, response, wallMs, err)
	return response, err
}

func (r Runner) appendAgentCall(request ports.AgentRequest, response ports.AgentResponse, wallMs int64, callErr error) {
	if r.RunID == "" {
		return
	}
	usage := response.Usage
	model := usage.Model
	if model == "" {
		model = request.Model
	}
	duration := usage.DurationMs
	if duration == 0 {
		duration = wallMs
	}
	attempt := request.Attempt
	if attempt < 1 {
		attempt = 1
	}
	record := AgentCallRecord{
		Timestamp:    time.Now().UTC(),
		RunID:        request.RunID,
		StepID:       request.StepID,
		AgentName:    request.AgentName,
		Model:        model,
		Attempt:      attempt,
		DurationMs:   duration,
		ExitCode:     usage.ExitCode,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
	if callErr != nil {
		record.Error = callErr.Error()
		if record.ExitCode == 0 {
			record.ExitCode = -1
		}
	}

	path := filepath.Join(r.ArtifactRoot, ".dft", "runs", r.RunID, "agent-calls.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	line, err := json.Marshal(record)
	if err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(line, '\n'))
}
