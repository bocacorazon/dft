package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bocacorazon/dft/internal/flow"
)

func TestAggregateAgentStatsSumsPerAgent(t *testing.T) {
	records := []flow.AgentCallRecord{
		{AgentName: "specify", Attempt: 1, DurationMs: 100, InputTokens: 10, OutputTokens: 5},
		{AgentName: "specify", Attempt: 2, DurationMs: 50, ExitCode: 1, Error: "boom"},
		{AgentName: "plan", Attempt: 1, DurationMs: 200, InputTokens: 20, OutputTokens: 8},
	}

	stats, total := aggregateAgentStats(records)
	if len(stats) != 2 {
		t.Fatalf("agent buckets = %d, want 2", len(stats))
	}
	if stats[0].Agent != "plan" {
		t.Fatalf("top agent = %q, want plan (highest duration)", stats[0].Agent)
	}
	byName := map[string]agentStat{}
	for _, s := range stats {
		byName[s.Agent] = s
	}
	if got := byName["specify"]; got.Calls != 2 || got.DurationMs != 150 || got.Errors != 1 {
		t.Fatalf("specify stat = %+v, want calls=2 duration=150 errors=1", got)
	}
	if total.Calls != 3 || total.DurationMs != 350 || total.Errors != 1 {
		t.Fatalf("total = %+v, want calls=3 duration=350 errors=1", total)
	}
}

func TestPrintAgentStatsIncludesTotalRow(t *testing.T) {
	records := []flow.AgentCallRecord{
		{AgentName: "specify", Attempt: 1, DurationMs: 100},
	}
	var buf bytes.Buffer
	printAgentStats(&buf, records)
	out := buf.String()
	if !strings.Contains(out, "specify") || !strings.Contains(out, "TOTAL") {
		t.Fatalf("output missing agent or total row:\n%s", out)
	}
}
