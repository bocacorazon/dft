package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bocacorazon/dft/internal/domain"
)

// JSONStore persists run manifests as plain files under .dft/runs.
type JSONStore struct {
	RootDir string
}

// Save writes a run manifest.
func (s JSONStore) Save(manifest domain.RunManifest) error {
	if manifest.ID == "" {
		return fmt.Errorf("run id is required")
	}
	path := s.manifestPath(manifest.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create run directory: %w", err)
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run manifest: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write run manifest: %w", err)
	}
	return nil
}

// Load reads a run manifest.
func (s JSONStore) Load(id string) (domain.RunManifest, error) {
	content, err := os.ReadFile(s.manifestPath(id))
	if err != nil {
		return domain.RunManifest{}, fmt.Errorf("read run manifest: %w", err)
	}
	var manifest domain.RunManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return domain.RunManifest{}, fmt.Errorf("parse run manifest: %w", err)
	}
	return manifest, nil
}

// List reads all manifests in stable order.
func (s JSONStore) List() ([]domain.RunManifest, error) {
	pattern := filepath.Join(s.RootDir, ".dft", "runs", "*", "manifest.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob run manifests: %w", err)
	}
	sort.Strings(matches)
	manifests := make([]domain.RunManifest, 0, len(matches))
	for _, match := range matches {
		content, err := os.ReadFile(match)
		if err != nil {
			return nil, fmt.Errorf("read run manifest %s: %w", match, err)
		}
		var manifest domain.RunManifest
		if err := json.Unmarshal(content, &manifest); err != nil {
			return nil, fmt.Errorf("parse run manifest %s: %w", match, err)
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func (s JSONStore) manifestPath(id string) string {
	return filepath.Join(s.RootDir, ".dft", "runs", id, "manifest.json")
}
