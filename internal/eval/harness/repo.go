package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// provisionRepos creates a bare remote and a cloned workspace.
func (h *Harness) provisionRepos(runID, demand string) (remoteDir string, workspaceDir string, err error) {
	baseTemp, err := os.MkdirTemp("", "dft-eval-"+runID+"-*")
	if err != nil {
		return "", "", err
	}

	remoteDir = filepath.Join(baseTemp, "remote.git")
	workspaceDir = filepath.Join(baseTemp, "workspace")

	// 1. Create bare remote
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		return "", "", err
	}
	if err := runIn(remoteDir, "git", "init", "--bare", "-b", "main"); err != nil {
		return "", "", err
	}

	// 2. Clone to workspace
	if err := runIn(baseTemp, "git", "clone", remoteDir, "workspace"); err != nil {
		return "", "", err
	}

	// Config Git
	runIn(workspaceDir, "git", "config", "user.name", "dft-eval")
	runIn(workspaceDir, "git", "config", "user.email", "eval@dft.local")

	// 3. Write README.md with demand
	readme := filepath.Join(workspaceDir, "README.md")
	content := fmt.Sprintf("# Evaluation Target\n\n**Demand:**\n%s\n", demand)
	if err := os.WriteFile(readme, []byte(content), 0644); err != nil {
		return "", "", err
	}
	
	// 4. Provision dft
	if err := runIn(workspaceDir, h.DftPath, "init"); err != nil {
		return "", "", err
	}

	// 5. Commit and Push
	runIn(workspaceDir, "git", "add", ".")
	runIn(workspaceDir, "git", "commit", "-m", "initial: provision repo and demand")
	if err := runIn(workspaceDir, "git", "push", "origin", "main"); err != nil {
		return "", "", err
	}

	return remoteDir, workspaceDir, nil
}

func runIn(dir, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func runInWithOutput(dir, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
