//go:build e2e

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/testutil/containers"
)

func TestE2E_AttachLastJoinsLiveRun(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", `echo e2e-live-output; sleep 10; exit 0`)
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	installScript(t, env)
	startBackgroundRun(t, env, repoPath, homeDir, binPath, "claude")
	runID := waitForRunningEvidence(t, env, repoPath)
	startAttachProcess(t, env, repoPath, homeDir, binPath, "last")

	if !waitForAttachedClient(env, "tessariq-"+runID, 10*time.Second) {
		t.Fatalf("attach client did not connect\nattach log:\n%s\nrun output:\n%s\nclients:\n%s", readAttachLog(t, env), readRunOutput(t, env), listClients(t, env))
	}
	require.Eventually(t, func() bool {
		return strings.Contains(readRunLog(t, env, filepath.Join(repoPath, ".tessariq", "runs", runID, "run.log")), "e2e-live-output")
	}, 5*time.Second, 200*time.Millisecond)
}

func TestE2E_AttachMissingTmuxShowsActionableGuidance(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	_, _, err := env.Exec(ctx, []string{"sh", "-c", "mkdir -p /work/bin && ln -sf $(command -v git) /work/bin/git"})
	require.NoError(t, err)

	code, output := runAttachInEnv(t, env, repoPath, homeDir, binPath, "last", "PATH=/work/bin")
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"tmux\" is missing or unavailable")
	require.Contains(t, output, "install or enable tmux, then retry")
}

func TestE2E_AttachMissingGitShowsActionableGuidance(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnv(t, bin, 0)
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	ctx := context.Background()
	_, _, err := env.Exec(ctx, []string{"sh", "-c", "mkdir -p /work/bin && ln -sf $(command -v tmux) /work/bin/tmux"})
	require.NoError(t, err)

	code, output := runAttachInEnv(t, env, repoPath, homeDir, binPath, "last", "PATH=/work/bin")
	require.NotEqual(t, 0, code)
	require.Contains(t, output, "required host prerequisite \"git\" is missing or unavailable")
	require.Contains(t, output, "install or enable git, then retry")
}

func TestE2E_AttachLastFailsCleanlyWithIncompleteIndex(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	ctx := context.Background()

	// Write only incomplete index entries (missing required fields) inside the container.
	cmd := fmt.Sprintf(`mkdir -p %s/.tessariq/runs && printf '{"run_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","state":"running"}\n' > %s/.tessariq/runs/index.jsonl`, repoPath, repoPath)
	execCmd(t, env, ctx, cmd, "write corrupt index")

	code, output := runAttachInEnv(t, env, repoPath, homeDir, binPath, "last", "")
	require.NotEqual(t, 0, code, "attach should fail with incomplete index")
	require.Contains(t, output, "no matching run found")
}

func startBackgroundRun(t *testing.T, env *containers.RunEnv, repoPath, homeDir, binPath, binaryName string) {
	t.Helper()

	ctx := context.Background()
	imgFlag := fmt.Sprintf("--image tessariq-test-agent-%s-%s", binaryName, testImageTag(t))
	cmd := fmt.Sprintf("tmux new-session -d -s attach-run-launch 'cd %s && HOME=%s %s run %s --egress open tasks/sample.md >/work/run-output.txt 2>&1'", repoPath, homeDir, binPath, imgFlag)
	code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err)
	require.Equal(t, 0, code, "start background run failed: %s", output)
	t.Cleanup(func() {
		_, _, _ = env.Exec(context.Background(), []string{"sh", "-c", "tmux kill-session -t attach-run-launch"})
	})
}

func waitForRunningEvidence(t *testing.T, env *containers.RunEnv, repoPath string) string {
	t.Helper()

	runsDir := filepath.Join(repoPath, ".tessariq", "runs")
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		ctx := context.Background()
		cmd := fmt.Sprintf("for d in %s/*; do [ -d \"$d\" ] || continue; if [ -f \"$d/status.json\" ] && grep -q '\"state\": \"running\"' \"$d/status.json\"; then basename \"$d\"; exit 0; fi; done; exit 1", runsDir)
		code, output, err := env.Exec(ctx, []string{"sh", "-c", cmd})
		if err == nil && code == 0 {
			runID := strings.TrimSpace(output)
			if runID != "" {
				return runID
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("run never reached running state\nrun output:\n%s", readRunOutput(t, env))
	return ""
}

func installScript(t *testing.T, env *containers.RunEnv) {
	t.Helper()

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "apk add --no-cache util-linux"})
	require.NoError(t, err)
	require.Equal(t, 0, code, "install script failed: %s", output)
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

func waitForAttachedClient(env *containers.RunEnv, sessionName string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx := context.Background()
		code, output, err := env.Exec(ctx, []string{"sh", "-c", "tmux list-clients -F '#{client_tty} #{session_name}'"})
		if err == nil && code == 0 && strings.Contains(output, sessionName) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
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

func readRunOutput(t *testing.T, env *containers.RunEnv) string {
	t.Helper()

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", "cat /work/run-output.txt 2>/dev/null || true"})
	require.NoError(t, err)
	if code != 0 {
		return output
	}
	return output
}

func readRunLog(t *testing.T, env *containers.RunEnv, path string) string {
	t.Helper()

	ctx := context.Background()
	code, output, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("cat %s 2>/dev/null || true", path)})
	require.NoError(t, err)
	if code != 0 {
		return output
	}
	return output
}
