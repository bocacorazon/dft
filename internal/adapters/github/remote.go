package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Adapter records GitHub remote-only operations.
type Adapter struct {
	RootDir string
	DryRun  bool
}

// PRRequest describes a pull request creation operation.
type PRRequest struct {
	RunID  string `json:"run_id"`
	StepID string `json:"step_id"`
	Head   string `json:"head"`
	Base   string `json:"base"`
	Title  string `json:"title"`
}

// PRRecord is the remote-only audit record for a PR operation.
type PRRecord struct {
	PRRequest
	RemoteOnly bool   `json:"remote_only"`
	Status     string `json:"status"`
}

// CreatePR records a PR creation operation. Non-dry-run execution is intentionally deferred.
func (a Adapter) CreatePR(_ context.Context, request PRRequest) (PRRecord, error) {
	if request.RunID == "" || request.StepID == "" {
		return PRRecord{}, fmt.Errorf("run id and step id are required")
	}
	status := "dry_run"
	if !a.DryRun {
		return PRRecord{}, fmt.Errorf("non-dry-run GitHub PR creation is not enabled")
	}
	record := PRRecord{PRRequest: request, RemoteOnly: true, Status: status}
	path := filepath.Join(a.RootDir, ".dft", "runs", request.RunID, "remote", request.StepID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return PRRecord{}, fmt.Errorf("create remote audit directory: %w", err)
	}
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return PRRecord{}, fmt.Errorf("encode remote audit record: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return PRRecord{}, fmt.Errorf("write remote audit record: %w", err)
	}
	return record, nil
}
