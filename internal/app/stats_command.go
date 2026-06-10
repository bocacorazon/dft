package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bocacorazon/dft/internal/flow"
)

// runStats prints per-agent observability for a run from its agent-calls.jsonl.
func runStats(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "stats requires a run id")
		return 2
	}
	runID := args[0]
	records, err := loadAgentCalls(runID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if len(records) == 0 {
		fmt.Fprintf(stdout, "run %s: no agent calls recorded\n", runID)
		return 0
	}
	fmt.Fprintf(stdout, "run: %s\n", runID)
	printAgentStats(stdout, records)
	return 0
}

// agentCallsPath is the durable JSONL of per-attempt agent invocations.
func agentCallsPath(runID string) string {
	return filepath.Join(".dft", "runs", runID, "agent-calls.jsonl")
}

func loadAgentCalls(runID string) ([]flow.AgentCallRecord, error) {
	file, err := os.Open(agentCallsPath(runID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open agent calls: %w", err)
	}
	defer file.Close()

	var records []flow.AgentCallRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record flow.AgentCallRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("parse agent call record: %w", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read agent calls: %w", err)
	}
	return records, nil
}

type agentStat struct {
	Agent        string
	Calls        int
	DurationMs   int64
	InputTokens  int
	OutputTokens int
	Errors       int
}

func aggregateAgentStats(records []flow.AgentCallRecord) ([]agentStat, agentStat) {
	byAgent := map[string]*agentStat{}
	var total agentStat
	total.Agent = "TOTAL"
	for _, record := range records {
		stat, ok := byAgent[record.AgentName]
		if !ok {
			stat = &agentStat{Agent: record.AgentName}
			byAgent[record.AgentName] = stat
		}
		stat.Calls++
		stat.DurationMs += record.DurationMs
		stat.InputTokens += record.InputTokens
		stat.OutputTokens += record.OutputTokens
		total.Calls++
		total.DurationMs += record.DurationMs
		total.InputTokens += record.InputTokens
		total.OutputTokens += record.OutputTokens
		if record.Error != "" || record.ExitCode != 0 {
			stat.Errors++
			total.Errors++
		}
	}
	stats := make([]agentStat, 0, len(byAgent))
	for _, stat := range byAgent {
		stats = append(stats, *stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].DurationMs != stats[j].DurationMs {
			return stats[i].DurationMs > stats[j].DurationMs
		}
		return stats[i].Agent < stats[j].Agent
	})
	return stats, total
}

func printAgentStats(w io.Writer, records []flow.AgentCallRecord) {
	stats, total := aggregateAgentStats(records)
	fmt.Fprintf(w, "%-32s %6s %12s %10s %10s %7s\n", "agent", "calls", "duration_ms", "tok_in", "tok_out", "errors")
	for _, stat := range stats {
		fmt.Fprintf(w, "%-32s %6d %12d %10d %10d %7d\n", stat.Agent, stat.Calls, stat.DurationMs, stat.InputTokens, stat.OutputTokens, stat.Errors)
	}
	fmt.Fprintf(w, "%-32s %6d %12d %10d %10d %7d\n", total.Agent, total.Calls, total.DurationMs, total.InputTokens, total.OutputTokens, total.Errors)
}
