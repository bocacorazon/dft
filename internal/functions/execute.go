package functions

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Execute(workDir string, fn string, args map[string]string) (string, map[string]string, error) {
	switch fn {
	case "set_var":
		name := strings.TrimSpace(args["name"])
		if name == "" {
			return "", nil, fmt.Errorf("set_var requires args.name")
		}
		value := args["value"]
		return value, map[string]string{name: value}, nil
	case "git_branch_current":
		out, err := gitOutput(workDir, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return "", nil, err
		}
		return strings.TrimSpace(out), nil, nil
	case "git_push":
		remote := args["remote"]
		if remote == "" {
			remote = "origin"
		}
		branch := args["branch"]
		if branch == "" {
			out, err := gitOutput(workDir, "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return "", nil, err
			}
			branch = strings.TrimSpace(out)
		}
		if err := gitRun(workDir, "push", remote, branch); err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("pushed %s %s", remote, branch), nil, nil
	case "wait_for_human":
		if os.Getenv("DFT_AUTOCONTINUE_WAIT_FOR_HUMAN") == "1" {
			return "auto-continued", nil, nil
		}
		return "", nil, fmt.Errorf("wait_for_human requested but no human interaction available")
	default:
		return "", nil, fmt.Errorf("unsupported function fn: %s", fn)
	}
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
