//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func TestIntegration_AttachLastJoinsLiveTmuxSession(t *testing.T) {
	t.Parallel()

	env := setupAttachIntegrationEnv(t)
	repoPath := filepath.Join(env.Dir(), "repo")
	homeDir := filepath.Join(env.Dir(), "home")
	binPath := filepath.Join(env.Dir(), "tessariq")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createIndexedRun(t, repoPath, runID, runner.NewInitialStatus(time.Now()))
	startSession(t, env, run.SessionName(runID), "printf integration-live-output; sleep 10")
	startAttachProcess(t, env, repoPath, homeDir, binPath, "last")

	require.Eventuallyf(t, func() bool {
		clients := listClients(t, env)
		return strings.Contains(clients, run.SessionName(runID))
	}, 5*time.Second, 100*time.Millisecond, "attach log: %s", readAttachLog(t, env))
	require.Contains(t, capturePane(t, env, run.SessionName(runID)), "integration-live-output")
}

func TestIntegration_AttachFinishedRunFailsWithoutAttaching(t *testing.T) {
	t.Parallel()

	env := setupAttachIntegrationEnv(t)
	repoPath := filepath.Join(env.Dir(), "repo")
	homeDir := filepath.Join(env.Dir(), "home")
	binPath := filepath.Join(env.Dir(), "tessariq")
	runID := "01ARZ3NDEKTSV4RRFFQ69G5FAA"

	createIndexedRun(t, repoPath, runID, runner.NewTerminalStatus(runner.StateSuccess, time.Now().Add(-time.Minute), time.Now(), 0, false))
	startSession(t, env, run.SessionName(runID), "printf should-not-attach; sleep 10")

	code, output := runAttachInEnv(t, env, repoPath, homeDir, binPath, runID, "")
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "run "+runID+" is not live")
	require.Contains(t, output, filepath.Join(repoPath, ".tessariq", "runs", runID))
	stopSession(t, env, run.SessionName(runID))
}

func TestIntegration_AttachLastFailsCleanlyWithIncompleteIndex(t *testing.T) {
	t.Parallel()

	env := setupAttachIntegrationEnv(t)
	repoPath := filepath.Join(env.Dir(), "repo")
	homeDir := filepath.Join(env.Dir(), "home")
	binPath := filepath.Join(env.Dir(), "tessariq")

	// Write only incomplete index entries (missing required fields).
	ctx := context.Background()
	incompleteIndex := `{"run_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","state":"running"}` + "\n"
	indexPath := filepath.Join(repoPath, ".tessariq", "runs", "index.jsonl")
	cmd := fmt.Sprintf("printf '%s' > %s", incompleteIndex, indexPath)
	code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err)
	require.Equal(t, 0, code, "write index: %s", output)

	code, output = runAttachInEnv(t, env, repoPath, homeDir, binPath, "last", "")
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "no matching run found")
}

func buildAttachIntegrationBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	bin := filepath.Join(dir, "tessariq")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/tessariq")
	cmd.Dir = findModuleRoot()
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build integration binary: %s", out)
	return bin
}

func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if dir == parent {
			panic("could not find go.mod")
		}
		dir = parent
	}
}

func setupAttachIntegrationEnv(t *testing.T) *containers.RunEnv {
	t.Helper()

	ctx := context.Background()
	env, err := containers.StartRunEnv(ctx, t, 0)
	require.NoError(t, err)

	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	binData, err := os.ReadFile(buildAttachIntegrationBinary(t))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(binPath, binData, 0o755))

	cmds := []string{
		"apk add --no-cache util-linux",
		fmt.Sprintf("mkdir -p %s", homeDir),
		fmt.Sprintf("mkdir -p %s/tasks", repoPath),
		fmt.Sprintf("git init %s", repoPath),
		fmt.Sprintf("git -C %s config user.email test@test.com", repoPath),
		fmt.Sprintf("git -C %s config user.name Test", repoPath),
		fmt.Sprintf("printf '# Sample Task\n\nDo something.\n' > %s/tasks/sample.md", repoPath),
		fmt.Sprintf("git -C %s add -A", repoPath),
		fmt.Sprintf("git -C %s commit -m initial", repoPath),
		fmt.Sprintf("cd %s && HOME=%s %s init", repoPath, homeDir, binPath),
		fmt.Sprintf("chmod -R a+rwX %s/.tessariq", repoPath),
	}
	for _, cmd := range cmds {
		code, output, execErr := env.Exec(ctx, []string{"sh", "-c", cmd})
		require.NoError(t, execErr, "exec %q: %s", cmd, output)
		require.Equal(t, 0, code, "exec %q exited %d: %s", cmd, code, output)
	}

	return env
}

func createIndexedRun(t *testing.T, repoPath, runID string, status runner.Status) {
	t.Helper()

	evidenceDir := filepath.Join(repoPath, ".tessariq", "runs", runID)
	manifest := run.Manifest{
		SchemaVersion: 1,
		RunID:         runID,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		TaskPath:      "tasks/sample.md",
		TaskTitle:     "Sample Task",
		Agent:         "claude-code",
		WorkspaceMode: "worktree",
	}
	require.NoError(t, run.WriteManifest(evidenceDir, manifest))
	require.NoError(t, runner.WriteStatus(evidenceDir, status))
	require.NoError(t, run.AppendIndex(filepath.Join(repoPath, ".tessariq", "runs"), run.IndexEntryFromManifest(manifest, string(status.State))))
}

func startSession(t *testing.T, env *containers.RunEnv, sessionName, command string) {
	t.Helper()

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("tmux new-session -d -s %s '%s'", sessionName, command)})
	require.NoError(t, err)
	require.Equal(t, 0, code, "start session failed: %s", output)
	t.Cleanup(func() { stopSession(t, env, sessionName) })
}

func stopSession(t *testing.T, env *containers.RunEnv, sessionName string) {
	t.Helper()

	ctx := context.Background()
	_, _, _ = env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("tmux kill-session -t %s", sessionName)})
}

func startAttachProcess(t *testing.T, env *containers.RunEnv, repoPath, homeDir, binPath, runRef string) {
	t.Helper()

	ctx := context.Background()
	cmd := fmt.Sprintf("cd %s && TERM=xterm HOME=%s script -q -c '%s attach %s' /dev/null >/work/attach-client.log 2>&1 &", repoPath, homeDir, binPath, runRef)
	code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err)
	require.Equal(t, 0, code, "start attach client failed: %s", output)
}

func capturePane(t *testing.T, env *containers.RunEnv, sessionName string) string {
	t.Helper()

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("tmux capture-pane -p -t %s", sessionName)})
	require.NoError(t, err)
	require.Equal(t, 0, code, "capture pane failed: %s", output)
	return output
}

func listClients(t *testing.T, env *containers.RunEnv) string {
	t.Helper()

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "tmux list-clients -F '#{client_tty} #{session_name}'"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "list clients failed: %s", output)
	return output
}

func runAttachInEnv(t *testing.T, env *containers.RunEnv, repoPath, homeDir, binPath, runRef, envPrefix string) (int, string) {
	t.Helper()

	ctx := context.Background()
	prefix := fmt.Sprintf("HOME=%s", homeDir)
	if envPrefix != "" {
		prefix = envPrefix + " " + prefix
	}
	cmd := fmt.Sprintf("cd %s && %s %s attach %s", repoPath, prefix, binPath, runRef)
	code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err)
	return code, output
}

func readAttachLog(t *testing.T, env *containers.RunEnv) string {
	t.Helper()

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cat /work/attach-client.log 2>/dev/null || true"})
	require.NoError(t, err)
	if code != 0 {
		return output
	}
	return output
}
