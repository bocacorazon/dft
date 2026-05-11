package verify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

type Check struct {
	Fn   string   `yaml:"fn" json:"fn"`
	Args []string `yaml:"args" json:"args"`
}

type Failure struct {
	StepID  string   `json:"step_id"`
	CheckFn string   `json:"check_fn"`
	Args    []string `json:"args"`
	Message string   `json:"message"`
}

func Evaluate(workDir string, stepID string, checks []Check) ([]Failure, error) {
	failures := make([]Failure, 0)
	for _, check := range checks {
		ok, message, err := runCheck(workDir, check)
		if err != nil {
			return nil, err
		}
		if !ok {
			failures = append(failures, Failure{
				StepID:  stepID,
				CheckFn: check.Fn,
				Args:    check.Args,
				Message: message,
			})
		}
	}
	return failures, nil
}

func runCheck(workDir string, check Check) (bool, string, error) {
	switch check.Fn {
	case "file_exists":
		if len(check.Args) != 1 {
			return false, "", fmt.Errorf("file_exists requires 1 arg")
		}
		target := filepath.Join(workDir, check.Args[0])
		if _, err := os.Stat(target); err == nil {
			return true, "", nil
		}
		return false, fmt.Sprintf("file does not exist: %s", check.Args[0]), nil
	case "command_exit_zero":
		if len(check.Args) < 1 {
			return false, "", fmt.Errorf("command_exit_zero requires command args")
		}
		cmd := exec.Command(check.Args[0], check.Args[1:]...)
		cmd.Dir = workDir
		if err := cmd.Run(); err != nil {
			return false, fmt.Sprintf("command failed: %v", err), nil
		}
		return true, "", nil
	case "grep_matches":
		if len(check.Args) != 2 {
			return false, "", fmt.Errorf("grep_matches requires pattern and file path")
		}
		pattern := check.Args[0]
		target := filepath.Join(workDir, check.Args[1])
		data, err := os.ReadFile(target)
		if err != nil {
			return false, fmt.Sprintf("read target file failed: %v", err), nil
		}
		matched, err := regexp.Match(pattern, data)
		if err != nil {
			return false, "", fmt.Errorf("invalid regex: %w", err)
		}
		if !matched {
			return false, fmt.Sprintf("pattern not found: %s", pattern), nil
		}
		return true, "", nil
	default:
		return false, "", fmt.Errorf("unsupported verify fn: %s", check.Fn)
	}
}
