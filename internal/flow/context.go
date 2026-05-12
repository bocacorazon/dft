package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type contextHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func attachProjectContext(root string, prompt string) (string, []contextHash, error) {
	contextDir := filepath.Join(root, ".dft", "context")
	entries, err := os.ReadDir(contextDir)
	if os.IsNotExist(err) {
		return prompt, nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("read project context: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(contextDir, entry.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return prompt, nil, nil
	}

	var builder strings.Builder
	builder.WriteString("Project context:\n")
	hashes := make([]contextHash, 0, len(files))
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", nil, fmt.Errorf("read context file %s: %w", path, err)
		}
		sum := sha256.Sum256(content)
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", nil, fmt.Errorf("relativize context file %s: %w", path, err)
		}
		builder.WriteString("\n--- ")
		builder.WriteString(filepath.ToSlash(relative))
		builder.WriteString(" ---\n")
		builder.Write(content)
		if len(content) == 0 || content[len(content)-1] != '\n' {
			builder.WriteByte('\n')
		}
		hashes = append(hashes, contextHash{Path: filepath.ToSlash(relative), SHA256: hex.EncodeToString(sum[:])})
	}
	builder.WriteString("\nTask prompt:\n")
	builder.WriteString(prompt)
	return builder.String(), hashes, nil
}

func writeContextHashes(stepDir string, hashes []contextHash) error {
	if len(hashes) == 0 {
		return nil
	}
	content, err := json.MarshalIndent(struct {
		Context []contextHash `json:"context"`
	}{Context: hashes}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode context hashes: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stepDir, "context-hashes.json"), append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write context hashes: %w", err)
	}
	return nil
}
