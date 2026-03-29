package workflow

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	RepoRoot     string
	PlanningDir  string
	StateFile    string
	TasksDir     string
	ArtifactsDir string
	AgentSkills  string
	ClaudeSkills string
	PrimarySpec  string
}

func DiscoverPaths(start string) (Paths, error) {
	root, err := findRepoRoot(start)
	if err != nil {
		return Paths{}, err
	}

	planning := filepath.Join(root, "planning")
	return Paths{
		RepoRoot:     root,
		PlanningDir:  planning,
		StateFile:    filepath.Join(planning, "STATE.md"),
		TasksDir:     filepath.Join(planning, "tasks"),
		ArtifactsDir: filepath.Join(planning, "artifacts"),
		AgentSkills:  filepath.Join(root, ".agents", "skills"),
		ClaudeSkills: filepath.Join(root, ".claude", "skills"),
		PrimarySpec:  filepath.Join(root, "specs", "tessariq-v0.1.0.md"),
	}, nil
}

func findRepoRoot(start string) (string, error) {
	dir := filepath.Clean(start)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("find repository root from %s", start)
		}
		dir = parent
	}
}
