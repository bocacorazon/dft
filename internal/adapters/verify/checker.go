package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
)

// Checker evaluates deterministic verification checks.
type Checker struct {
	RootDir string
}

// Run evaluates checks in order and returns all results and findings.
func (c Checker) Run(ctx context.Context, checks []domain.Check) domain.VerificationResult {
	result := domain.VerificationResult{
		Status:  domain.VerdictPass,
		Results: make([]domain.CheckResult, 0, len(checks)),
	}

	for _, check := range checks {
		checkResult := c.runOne(ctx, check)
		result.Results = append(result.Results, checkResult)
		if !checkResult.Passed {
			result.Status = domain.VerdictFail
			result.Findings = append(result.Findings, domain.Finding{
				CheckID: check.ID,
				Message: checkResult.Message,
			})
		}
	}

	return result
}

func (c Checker) runOne(ctx context.Context, check domain.Check) domain.CheckResult {
	if check.ID == "" {
		return failed(check.ID, "check id is required")
	}

	switch check.Kind {
	case domain.CheckFileExists:
		if len(check.Args) != 1 {
			return failed(check.ID, "file_exists requires path argument")
		}
		path := c.path(check.Args[0])
		if _, err := os.Stat(path); err != nil {
			return failed(check.ID, fmt.Sprintf("expected file %s to exist: %v", check.Args[0], err))
		}
		return passed(check.ID)
	case domain.CheckFileMissing:
		if len(check.Args) != 1 {
			return failed(check.ID, "file_missing requires path argument")
		}
		path := c.path(check.Args[0])
		if _, err := os.Stat(path); err == nil {
			return failed(check.ID, fmt.Sprintf("expected file %s to be missing", check.Args[0]))
		} else if !os.IsNotExist(err) {
			return failed(check.ID, fmt.Sprintf("stat %s: %v", check.Args[0], err))
		}
		return passed(check.ID)
	case domain.CheckGrepMatches:
		if len(check.Args) != 2 {
			return failed(check.ID, "grep_matches requires path and substring arguments")
		}
		content, err := os.ReadFile(c.path(check.Args[0]))
		if err != nil {
			return failed(check.ID, fmt.Sprintf("read %s: %v", check.Args[0], err))
		}
		if !strings.Contains(string(content), check.Args[1]) {
			return failed(check.ID, fmt.Sprintf("%s does not contain %q", check.Args[0], check.Args[1]))
		}
		return passed(check.ID)
	case domain.CheckCommandExitZero:
		if len(check.Args) == 0 {
			return failed(check.ID, "command_exit_zero requires argv")
		}
		cmd := exec.CommandContext(ctx, check.Args[0], check.Args[1:]...)
		cmd.Dir = c.RootDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = err.Error()
			}
			return failed(check.ID, message)
		}
		return passed(check.ID)
	case domain.CheckJSONPathEquals:
		if len(check.Args) != 3 {
			return failed(check.ID, "json_path_equals requires path, json path, and expected value arguments")
		}
		content, err := os.ReadFile(c.path(check.Args[0]))
		if err != nil {
			return failed(check.ID, fmt.Sprintf("read %s: %v", check.Args[0], err))
		}
		var document any
		if err := json.Unmarshal(content, &document); err != nil {
			return failed(check.ID, fmt.Sprintf("parse %s: %v", check.Args[0], err))
		}
		value, ok := lookupJSONPath(document, check.Args[1])
		if !ok {
			return failed(check.ID, fmt.Sprintf("json path %s not found", check.Args[1]))
		}
		if got := fmt.Sprint(value); got != check.Args[2] {
			return failed(check.ID, fmt.Sprintf("json path %s = %q, want %q", check.Args[1], got, check.Args[2]))
		}
		return passed(check.ID)
	default:
		return failed(check.ID, fmt.Sprintf("unsupported check kind %q", check.Kind))
	}
}

func (c Checker) path(path string) string {
	if filepath.IsAbs(path) || c.RootDir == "" {
		return path
	}
	return filepath.Join(c.RootDir, path)
}

func passed(id string) domain.CheckResult {
	return domain.CheckResult{CheckID: id, Passed: true}
}

func failed(id string, message string) domain.CheckResult {
	return domain.CheckResult{CheckID: id, Passed: false, Message: message}
}

func lookupJSONPath(document any, path string) (any, bool) {
	current := document
	for _, segment := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
