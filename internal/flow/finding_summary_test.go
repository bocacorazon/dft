package flow

import "testing"

func TestParseAnalyzeOutputTreatsPrerequisiteFailureAsBlocking(t *testing.T) {
	output, err := parseAnalyzeOutput("Prerequisite check failed: Not on a feature branch")
	if err != nil {
		t.Fatalf("parseAnalyzeOutput returned error: %v", err)
	}
	summary, ok := output["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary = %#v, want map", output["summary"])
	}
	if got := summary["blocking_findings"]; got != 1 {
		t.Fatalf("blocking_findings = %#v, want 1", got)
	}
}
