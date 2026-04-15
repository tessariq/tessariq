//go:build e2e

package main

import (
	"context"
	"encoding/json"
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
	// Sleep 30s so the agent stays live long enough for attach to race in
	// even on slow CI runners. The background run is launched with
	// --no-update-agent (see startBackgroundRun) to skip the init-container
	// version probe, which would otherwise run this same fake script for an
	// extra ~10s before the main run even starts.
	env := setupRunEnvWithScript(t, bin, "claude", `echo e2e-live-output; sleep 30; exit 0`)
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")

	installScript(t, env)
	startBackgroundRun(t, env, repoPath, homeDir, binPath, "claude")
	runID := waitForRunningEvidence(t, env, repoPath)
	// Once we know the runID, register an explicit agent-container cleanup
	// so the sibling container does not linger on the host daemon for the
	// remainder of the 60s sleep after the test returns.
	t.Cleanup(func() {
		_, _, _ = env.Exec(context.Background(), []string{"sh", "-c", fmt.Sprintf("docker rm -f tessariq-%s 2>/dev/null || true", runID)})
	})
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

func TestE2E_AttachForgedCrossRunEvidenceRejectsBeforeAttaching(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvCustom(t, bin, e2eSetupOpts{skipImage: true})
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	ctx := context.Background()

	runA := "01ARZ3NDEKTSV4RRFFQ69G5FAC"
	runB := "01ARZ3NDEKTSV4RRFFQ69G5FAD"

	// Create RUN_B with live evidence inside the container.
	evidenceDir := filepath.Join(repoPath, ".tessariq", "runs", runB)
	execCmd(t, env, ctx, fmt.Sprintf("mkdir -p %s", evidenceDir), "create evidence dir")
	execCmd(t, env, ctx, fmt.Sprintf(`printf '{"state":"running","started_at":"2026-01-01T00:00:00Z"}' > %s/status.json`, evidenceDir), "write status")

	// Write RUN_B's own index entry and a forged entry for RUN_A pointing at RUN_B's evidence.
	indexPath := filepath.Join(repoPath, ".tessariq", "runs", "index.jsonl")
	entryB := fmt.Sprintf(`{"run_id":"%s","created_at":"2026-01-01T00:00:00Z","task_path":"tasks/sample.md","task_title":"B","agent":"claude-code","workspace_mode":"worktree","state":"running","evidence_path":".tessariq/runs/%s"}`, runB, runB)
	entryA := fmt.Sprintf(`{"run_id":"%s","created_at":"2026-01-01T00:01:00Z","task_path":"tasks/sample.md","task_title":"Forged A","agent":"claude-code","workspace_mode":"worktree","state":"running","evidence_path":".tessariq/runs/%s"}`, runA, runB)
	execCmd(t, env, ctx, fmt.Sprintf("printf '%s\\n%s\\n' > %s", entryB, entryA, indexPath), "write forged index")

	// Start a tmux session for RUN_B so the liveness check would pass if not for the guard.
	execCmd(t, env, ctx, fmt.Sprintf("tmux new-session -d -s tessariq-%s 'sleep 30'", runB), "start tmux session")
	t.Cleanup(func() {
		_, _, _ = env.Exec(context.Background(), []string{"sh", "-c", fmt.Sprintf("tmux kill-session -t tessariq-%s", runB)})
	})

	code, output := runAttachInEnv(t, env, repoPath, homeDir, binPath, runA, "")
	require.NotEqual(t, 0, code, "attach should fail for cross-run evidence forgery")
	require.Contains(t, output, "run "+runA+" is not live")
	require.Contains(t, output, "run_id mismatch")
}

func TestE2E_AttachReconcilesExitedOrphanedRun(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	env := setupRunEnvWithScript(t, bin, "claude", "sleep 2; echo orphan-recovered; exit 0")
	hostDir := env.Dir()
	repoPath := filepath.Join(hostDir, "repo")
	homeDir := filepath.Join(hostDir, "home")
	binPath := filepath.Join(hostDir, "tessariq")
	ctx := context.Background()

	imgFlag := fmt.Sprintf("--image tessariq-test-agent-%s-%s", "claude", testImageTag(t))
	launchCmd := fmt.Sprintf("cd %s && HOME=%s %s run %s tasks/sample.md >/work/orphan-run.log 2>&1 & echo $! >/work/orphan-run.pid", repoPath, homeDir, binPath, imgFlag)
	execCmd(t, env, ctx, launchCmd, "launch orphanable run")

	runID := waitForRunningEvidence(t, env, repoPath)
	containerName := "tessariq-" + runID
	execCmd(t, env, ctx, "kill -9 $(cat /work/orphan-run.pid)", "kill host tessariq process")

	require.Eventually(t, func() bool {
		code, output, err := env.Exec(context.Background(), []string{"sh", "-c", fmt.Sprintf("docker inspect -f '{{.State.Running}} {{.State.ExitCode}}' %s 2>/dev/null || true", containerName)})
		return err == nil && code == 0 && strings.Contains(output, "false 0")
	}, 10*time.Second, 250*time.Millisecond, "orphaned agent container must exit successfully")

	code, output := runAttachInEnv(t, env, repoPath, homeDir, binPath, "last", "")
	require.NotEqual(t, 0, code, "attach should refuse a reconciled terminal run")
	require.Contains(t, output, "run "+runID+" is not live")
	require.Contains(t, output, "state success")

	evidencePath := filepath.Join(repoPath, ".tessariq", "runs", runID)
	statusCode, statusData, err := env.Exec(ctx, []string{"cat", filepath.Join(evidencePath, "status.json")})
	require.NoError(t, err)
	require.Equal(t, 0, statusCode, "status.json must exist")

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(statusData), &status))
	require.Equal(t, "success", status["state"])

	indexEntry := readLastIndexEntry(t, env, repoPath)
	require.Equal(t, runID, indexEntry["run_id"])
	require.Equal(t, "success", indexEntry["state"])

	inspectCode, _, err := env.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("docker inspect %s >/dev/null 2>&1", containerName)})
	require.NoError(t, err)
	require.NotEqual(t, 0, inspectCode, "reconciliation must remove the stale exited container")
}

func startBackgroundRun(t *testing.T, env *containers.RunEnv, repoPath, homeDir, binPath, binaryName string) {
	t.Helper()

	ctx := context.Background()
	imgFlag := fmt.Sprintf("--image tessariq-test-agent-%s-%s", binaryName, testImageTag(t))
	// --no-update-agent skips the agent-update init container. Without this
	// flag, the fake claude script is invoked once by the version probe
	// (which ignores --version and runs the full `sleep N; exit 0` body),
	// burning ~N seconds before the real run even begins. It also avoids a
	// noisy "exec: npm not found" warning from the npm-based update command.
	cmd := fmt.Sprintf("tmux new-session -d -s attach-run-launch 'cd %s && HOME=%s %s run %s --no-update-agent --egress open tasks/sample.md >/work/run-output.txt 2>&1'", repoPath, homeDir, binPath, imgFlag)
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
