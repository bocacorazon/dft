package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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
	case domain.CheckCountMatchesAtLeast:
		if len(check.Args) != 3 {
			return failed(check.ID, "count_matches_at_least requires path, substring, and minimum count arguments")
		}
		content, err := os.ReadFile(c.path(check.Args[0]))
		if err != nil {
			return failed(check.ID, fmt.Sprintf("read %s: %v", check.Args[0], err))
		}
		minimum, err := strconv.Atoi(check.Args[2])
		if err != nil || minimum < 0 {
			return failed(check.ID, fmt.Sprintf("minimum count must be a non-negative integer: %q", check.Args[2]))
		}
		count := strings.Count(string(content), check.Args[1])
		if count < minimum {
			return failed(check.ID, fmt.Sprintf("%s contains %q %d time(s), want at least %d", check.Args[0], check.Args[1], count, minimum))
		}
		return passed(check.ID)
	case domain.CheckOS:
		if len(check.Args) == 0 {
			return failed(check.ID, "os requires at least one allowed GOOS argument")
		}
		for _, allowed := range check.Args {
			if allowed == runtime.GOOS {
				return passed(check.ID)
			}
		}
		return failed(check.ID, fmt.Sprintf("runtime GOOS %q is not one of %q", runtime.GOOS, strings.Join(check.Args, ",")))
	case domain.CheckNoBinaryArtifacts:
		if len(check.Args) != 0 {
			return failed(check.ID, "no_binary_artifacts does not accept arguments")
		}
		artifacts, err := c.binaryArtifacts(ctx)
		if err != nil {
			return failed(check.ID, err.Error())
		}
		if len(artifacts) > 0 {
			return failed(check.ID, "tracked binary artifacts are not allowed: "+strings.Join(artifacts, ", "))
		}
		return passed(check.ID)
	default:
		return failed(check.ID, fmt.Sprintf("unsupported check kind %q", check.Kind))
	}
}

func (c Checker) binaryArtifacts(ctx context.Context) ([]string, error) {
	paths, err := c.trackedPaths(ctx)
	if err != nil {
		paths, err = c.walkSourcePaths()
		if err != nil {
			return nil, err
		}
	}
	artifacts := make([]string, 0)
	for _, path := range paths {
		if isIgnoredArtifactPath(path) {
			continue
		}
		info, err := os.Stat(c.path(path))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		if info.IsDir() {
			continue
		}
		binary, err := isBinaryArtifact(c.path(path), info)
		if err != nil {
			return nil, err
		}
		if binary {
			artifacts = append(artifacts, path)
		}
	}
	sort.Strings(artifacts)
	return artifacts, nil
}

func (c Checker) trackedPaths(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z")
	cmd.Dir = c.RootDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	parts := bytes.Split(output, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		paths = append(paths, string(part))
	}
	return paths, nil
}

func (c Checker) walkSourcePaths() ([]string, error) {
	var paths []string
	root := c.RootDir
	if root == "" {
		root = "."
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() && (name == ".git" || name == ".dft") {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk source files: %w", err)
	}
	return paths, nil
}

func isIgnoredArtifactPath(path string) bool {
	clean := filepath.ToSlash(path)
	return strings.HasPrefix(clean, ".dft/") || strings.HasPrefix(clean, ".git/")
}

func isBinaryArtifact(path string, info os.FileInfo) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if hasExecutableMagic(content) {
		return true, nil
	}
	if info.Mode()&0o111 != 0 && info.Size() > 1024*1024 {
		return true, nil
	}
	return false, nil
}

func hasExecutableMagic(content []byte) bool {
	if len(content) >= 4 && bytes.Equal(content[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return true
	}
	if len(content) >= 2 && bytes.Equal(content[:2], []byte{'M', 'Z'}) {
		return true
	}
	if len(content) < 4 {
		return false
	}
	switch string(content[:4]) {
	case "\xfe\xed\xfa\xce", "\xce\xfa\xed\xfe", "\xfe\xed\xfa\xcf", "\xcf\xfa\xed\xfe", "\xca\xfe\xba\xbe", "\xbe\xba\xfe\xca":
		return true
	default:
		return false
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
