package flow

import (
	"fmt"
	"strings"
)

func normalizeJSONStepOutput(parsed any, raw string) map[string]any {
	output := map[string]any{
		"stdout": raw,
	}
	switch value := parsed.(type) {
	case map[string]any:
		for key, item := range value {
			output[key] = item
		}
	default:
		output["parsed"] = value
	}
	if summary, ok := summarizeFindings(output["findings"]); ok {
		output["summary"] = summary
	} else {
		output["summary"] = map[string]any{
			"critical_findings": 0,
			"high_findings":     0,
			"medium_findings":   0,
			"low_findings":      0,
			"blocking_findings": 0,
			"total_findings":    0,
		}
	}
	return output
}

func parseAnalyzeOutput(raw string) (map[string]any, error) {
	findings := make([]map[string]any, 0)
	for _, line := range strings.Split(raw, "\n") {
		fields := parseMarkdownRow(line)
		if len(fields) < 6 {
			continue
		}
		severity := normalizeSeverity(fields[2])
		if severity == "" {
			continue
		}
		findings = append(findings, map[string]any{
			"finding_id":     fields[0],
			"category":       fields[1],
			"severity":       severity,
			"location":       fields[3],
			"message":        fields[4],
			"recommendation": fields[5],
		})
	}
	output := map[string]any{
		"stdout":   raw,
		"findings": make([]any, len(findings)),
	}
	for i, finding := range findings {
		output["findings"].([]any)[i] = finding
	}
	if len(findings) == 0 && analyzeOutputIndicatesFailure(raw) {
		output["findings"] = []any{map[string]any{
			"finding_id": "analysis-abort",
			"category":   "analysis",
			"severity":   "CRITICAL",
			"message":    "speckit.analyze aborted before producing a complete findings report",
		}}
	}
	summary, _ := summarizeFindings(output["findings"])
	output["summary"] = summary
	return output, nil
}

func analyzeOutputIndicatesFailure(raw string) bool {
	lower := strings.ToLower(raw)
	for _, needle := range []string{
		"abort:",
		"error:",
		"prerequisite check failed",
		"not on a feature branch",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func parseMarkdownRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return nil
	}
	parts := strings.Split(trimmed, "|")
	fields := make([]string, 0, len(parts)-2)
	for _, part := range parts[1 : len(parts)-1] {
		value := strings.TrimSpace(part)
		if value == "" {
			fields = append(fields, "")
			continue
		}
		if strings.Trim(value, "-: ") == "" {
			return nil
		}
		fields = append(fields, value)
	}
	return fields
}

func summarizeFindings(value any) (map[string]any, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	summary := map[string]any{
		"critical_findings": 0,
		"high_findings":     0,
		"medium_findings":   0,
		"low_findings":      0,
		"blocking_findings": 0,
		"total_findings":    len(items),
	}
	for _, item := range items {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch normalizeSeverity(fmt.Sprint(finding["severity"])) {
		case "CRITICAL":
			summary["critical_findings"] = summary["critical_findings"].(int) + 1
			summary["blocking_findings"] = summary["blocking_findings"].(int) + 1
		case "HIGH":
			summary["high_findings"] = summary["high_findings"].(int) + 1
			summary["blocking_findings"] = summary["blocking_findings"].(int) + 1
		case "MEDIUM":
			summary["medium_findings"] = summary["medium_findings"].(int) + 1
		case "LOW":
			summary["low_findings"] = summary["low_findings"].(int) + 1
		default:
			summary["high_findings"] = summary["high_findings"].(int) + 1
			summary["blocking_findings"] = summary["blocking_findings"].(int) + 1
		}
	}
	return summary, true
}

func normalizeSeverity(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "CRITICAL", "HIGH", "MEDIUM", "LOW":
		return value
	default:
		return ""
	}
}
