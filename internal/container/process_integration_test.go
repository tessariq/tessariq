//go:build integration

package container_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/testutil"
)

func cleanupContainer(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	})
}

func TestContainerLifecycle_CreateStartWait(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "exit 0"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestContainerLifecycle_NonZeroExit(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "exit 42"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 42, code)
}

func TestContainerLifecycle_CleanupAfterWait(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"true"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	_, err = p.Wait()
	require.NoError(t, err)
	require.NoError(t, p.Cleanup(context.Background()))

	// Container should be removed after explicit cleanup.
	out, inspectErr := exec.Command("docker", "inspect", name).CombinedOutput()
	require.Error(t, inspectErr, "container should not exist after Cleanup: %s", string(out))
}

func TestContainerLifecycle_CleanupIsIdempotent(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"true"},
	})

	require.NoError(t, p.Start(t.Context()))

	_, err := p.Wait()
	require.NoError(t, err)

	require.NoError(t, p.Cleanup(context.Background()), "first Cleanup must succeed")
	require.NoError(t, p.Cleanup(context.Background()), "second Cleanup must be a no-op, not an error")
}

func TestContainerLifecycle_MountVisibility(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	testFile := filepath.Join(hostDir, "hello.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("from host"), 0o644))

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"cat", "/work/hello.txt"},
		WorkDir: "/work",
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/work", ReadOnly: false},
		},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestContainerLifecycle_MountWriteFromContainer(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	// container.Process no longer world-writes bind-mount sources — workspace
	// provisioning does that for worktrees. For this isolated container test,
	// open the scratch dir explicitly so the cap-dropped container user can
	// write without needing a real worktree.
	require.NoError(t, os.Chmod(hostDir, 0o777))

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo container-wrote > /work/output.txt"},
		WorkDir: "/work",
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/work", ReadOnly: false},
		},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)

	content, err := os.ReadFile(filepath.Join(hostDir, "output.txt"))
	require.NoError(t, err)
	require.Equal(t, "container-wrote\n", string(content))
}

func TestContainerLifecycle_ReadOnlyMount(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(hostDir, "read.txt"), []byte("ro"), 0o644))

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo fail > /data/write.txt"},
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/data", ReadOnly: true},
		},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "writing to read-only mount should fail")
}

func TestContainerLifecycle_DeterministicName(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"true"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	// Verify the container exists with the expected name.
	out, err := exec.Command("docker", "inspect", "--format={{.Name}}", name).Output()
	require.NoError(t, err)
	require.Contains(t, string(out), name)

	_, _ = p.Wait()
}

func TestContainerLifecycle_EnvVarsVisible(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	require.NoError(t, os.Chmod(hostDir, 0o777))

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo $MY_VAR > /work/env.txt"},
		WorkDir: "/work",
		Env:     map[string]string{"MY_VAR": "hello-from-env"},
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/work", ReadOnly: false},
		},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)

	content, err := os.ReadFile(filepath.Join(hostDir, "env.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello-from-env\n", string(content))
}

func TestContainerLifecycle_SignalStop(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	// Use a command that explicitly handles SIGTERM as PID 1.
	// Bare "sleep" as PID 1 ignores SIGTERM due to kernel PID-1 protection.
	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "trap 'exit 143' TERM; sleep 300 & wait"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	// Give container time to start.
	time.Sleep(500 * time.Millisecond)

	// Send SIGTERM via docker kill --signal=SIGTERM (non-blocking).
	err = p.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 143, code, "SIGTERM-handled container should exit with 143")
}

// TestContainerLifecycle_SIGTERMIsNonBlocking verifies that Signal(SIGTERM) returns
// immediately without waiting for the container to exit. A container that traps
// SIGTERM stays running after the signal, proving the call is non-blocking.
func TestContainerLifecycle_SIGTERMIsNonBlocking(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	// Container traps SIGTERM and keeps running.
	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "trap '' TERM; sleep 300"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Signal must return quickly (non-blocking).
	start := time.Now()
	err = p.Signal(syscall.SIGTERM)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Less(t, elapsed, 3*time.Second,
		"Signal(SIGTERM) must be non-blocking, took %s", elapsed)

	// Container should still be running after SIGTERM since it traps the signal.
	out, inspectErr := exec.Command("docker", "inspect", "--format={{.State.Running}}", name).Output()
	require.NoError(t, inspectErr)
	require.Contains(t, string(out), "true", "container must still be running after trapped SIGTERM")

	// Clean up: SIGKILL to stop the container.
	_ = p.Signal(syscall.SIGKILL)
	_, _ = p.Wait()
}

func TestContainerLifecycle_SignalKill(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sleep", "300"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	err = p.Signal(syscall.SIGKILL)
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "killed container should have non-zero exit code")
}

func TestContainerLifecycle_DroppedCapabilities(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	require.NoError(t, os.Chmod(hostDir, 0o777))

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "grep CapEff /proc/1/status > /out/caps.txt"},
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/out", ReadOnly: false},
		},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)

	content, err := os.ReadFile(filepath.Join(hostDir, "caps.txt"))
	require.NoError(t, err)
	require.Contains(t, string(content), "0000000000000000",
		"effective capabilities must be zero with --cap-drop=ALL")
}

func TestContainerLifecycle_NoNewPrivileges(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	require.NoError(t, os.Chmod(hostDir, 0o777))

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "grep NoNewPrivs /proc/1/status > /out/privs.txt"},
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/out", ReadOnly: false},
		},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)

	content, err := os.ReadFile(filepath.Join(hostDir, "privs.txt"))
	require.NoError(t, err)
	require.Contains(t, string(content), "NoNewPrivs:\t1",
		"no-new-privileges must be enabled")
}

func TestContainerLifecycle_WorkDir(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	require.NoError(t, os.Chmod(hostDir, 0o777))

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "pwd > /out/pwd.txt"},
		WorkDir: "/myworkdir",
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/out", ReadOnly: false},
		},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)

	content, err := os.ReadFile(filepath.Join(hostDir, "pwd.txt"))
	require.NoError(t, err)
	require.Equal(t, "/myworkdir\n", string(content))
}

func TestContainerLifecycle_LogStreamNormalExit(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	var stdout bytes.Buffer
	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo hello-logs"},
	})
	p.SetOutputWriter(&stdout, io.Discard)

	err := p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)

	require.Contains(t, stdout.String(), "hello-logs",
		"normal exit must capture all log output")
}

func TestContainerLifecycle_LogStreamSurvivesContextCancel(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	name := testutil.UniqueName(t)
	cleanupContainer(t, name)

	var stdout bytes.Buffer
	p := container.New(container.Config{
		Name:  name,
		Image: "alpine:latest",
		// Trap SIGTERM and print shutdown output before exiting.
		Command: []string{"sh", "-c", "echo before-cancel; trap 'echo shutdown-output; exit 0' TERM; sleep 300 & wait"},
	})
	p.SetOutputWriter(&stdout, io.Discard)

	ctx, cancel := context.WithCancel(context.Background())
	err := p.Start(ctx)
	require.NoError(t, err)

	// Let container start and emit initial output.
	time.Sleep(500 * time.Millisecond)

	// Cancel context — simulates timeout context expiry.
	cancel()

	// Container is still running. Send SIGTERM to trigger shutdown output.
	time.Sleep(200 * time.Millisecond)
	err = p.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code)

	output := stdout.String()
	require.Contains(t, output, "before-cancel",
		"pre-cancel output must be captured")
	require.Contains(t, output, "shutdown-output",
		"post-cancel shutdown output must be captured in run.log")
}
