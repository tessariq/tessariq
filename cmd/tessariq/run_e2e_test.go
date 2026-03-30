//go:build e2e

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/tmux"
)

func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tessariq")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/tessariq")
	cmd.Dir = findModuleRoot(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", out)
	return bin
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "could not find go.mod")
		dir = parent
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git command %v failed: %s", args, out)
	}

	// Create a task file and commit.
	taskDir := filepath.Join(dir, "tasks")
	require.NoError(t, os.MkdirAll(taskDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "sample.md"), []byte("# Sample Task\n\nDo something.\n"), 0o644))

	commands = [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", "initial"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git command %v failed: %s", args, out)
	}

	return dir
}

func TestE2E_DetachedRunPrintsGuidance(t *testing.T) {
	skipIfNoTmux(t)

	bin := buildBinary(t)
	repo := initGitRepo(t)

	// Run tessariq init first.
	initCmd := exec.Command(bin, "init")
	initCmd.Dir = repo
	out, err := initCmd.CombinedOutput()
	require.NoError(t, err, "init failed: %s", out)

	// Run tessariq run with the task file.
	ctx := context.Background()
	runCmd := exec.CommandContext(ctx, bin, "run", "tasks/sample.md")
	runCmd.Dir = repo
	stdout, err := runCmd.Output()
	require.NoError(t, err, "run failed: %s", stdout)

	output := string(stdout)

	// Extract run_id from output to clean up tmux session.
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "run_id: ") {
			runID := strings.TrimPrefix(line, "run_id: ")
			sessionName := "tessariq-" + runID
			t.Cleanup(func() { _ = tmux.KillSession(ctx, sessionName) })
			break
		}
	}

	// Assert all six required fields are present.
	require.Contains(t, output, "run_id: ")
	require.Contains(t, output, "evidence_path: ")
	require.Contains(t, output, "workspace_path: ")
	require.Contains(t, output, "container_name: ")
	require.Contains(t, output, "attach: tessariq attach ")
	require.Contains(t, output, "promote: tessariq promote ")
}
