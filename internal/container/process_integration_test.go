//go:build integration

package container_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/container"
)

func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
}

func uniqueName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("tessariq-test-%d", time.Now().UnixNano())
}

func cleanupContainer(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	})
}

func TestContainerLifecycle_CreateStartWait(t *testing.T) {
	t.Parallel()
	skipIfNoDocker(t)

	name := uniqueName(t)
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
	skipIfNoDocker(t)

	name := uniqueName(t)
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
	skipIfNoDocker(t)

	name := uniqueName(t)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"true"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	_, err = p.Wait()
	require.NoError(t, err)

	// Container should be removed after Wait.
	out, inspectErr := exec.Command("docker", "inspect", name).CombinedOutput()
	require.Error(t, inspectErr, "container should not exist after Wait: %s", string(out))
}

func TestContainerLifecycle_MountVisibility(t *testing.T) {
	t.Parallel()
	skipIfNoDocker(t)

	name := uniqueName(t)
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
	skipIfNoDocker(t)

	name := uniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()

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
	skipIfNoDocker(t)

	name := uniqueName(t)
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
	skipIfNoDocker(t)

	name := uniqueName(t)
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
	skipIfNoDocker(t)

	name := uniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()

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

// TestContainerLifecycle_NonRootUserCanWriteAfterPrepare verifies that a
// non-root container user (tessariq) can write to a bind-mounted directory
// after prepareWritableMounts makes it world-writable.
func TestContainerLifecycle_NonRootUserCanWriteAfterPrepare(t *testing.T) {
	t.Parallel()
	skipIfNoDocker(t)

	// Build a minimal image with a tessariq user.
	imgName := fmt.Sprintf("tessariq-test-nonroot-%d", time.Now().UnixNano())
	buildCmd := exec.Command("docker", "build", "-t", imgName, "-f", "-", ".")
	buildCmd.Stdin = strings.NewReader(`FROM alpine:latest
RUN addgroup -S tessariq && adduser -S tessariq -G tessariq -h /home/tessariq
USER tessariq
`)
	out, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "image build failed: %s", out)
	t.Cleanup(func() { _ = exec.Command("docker", "rmi", "-f", imgName).Run() })

	name := uniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()
	// Create a file owned by the current (host) user with restrictive permissions.
	require.NoError(t, os.WriteFile(filepath.Join(hostDir, "existing.txt"), []byte("original"), 0o644))

	p := container.New(container.Config{
		Name:    name,
		Image:   imgName,
		Command: []string{"sh", "-c", "echo written-by-tessariq > /work/output.txt && cat /work/existing.txt"},
		WorkDir: "/work",
		User:    "tessariq",
		Mounts: []container.Mount{
			{Source: hostDir, Target: "/work", ReadOnly: false},
		},
	})

	// Start calls prepareWritableMounts internally.
	err = p.Start(t.Context())
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.Equal(t, 0, code, "non-root user should be able to write after prepare")

	// Verify the file was written by the container's tessariq user.
	content, err := os.ReadFile(filepath.Join(hostDir, "output.txt"))
	require.NoError(t, err)
	require.Equal(t, "written-by-tessariq\n", string(content))
}

func TestContainerLifecycle_SignalStop(t *testing.T) {
	t.Parallel()
	skipIfNoDocker(t)

	name := uniqueName(t)
	cleanupContainer(t, name)

	p := container.New(container.Config{
		Name:    name,
		Image:   "alpine:latest",
		Command: []string{"sleep", "300"},
	})

	err := p.Start(t.Context())
	require.NoError(t, err)

	// Give container time to start.
	time.Sleep(500 * time.Millisecond)

	// Send SIGTERM -> docker stop.
	err = p.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	code, err := p.Wait()
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "stopped container should have non-zero exit code")
}

func TestContainerLifecycle_SignalKill(t *testing.T) {
	t.Parallel()
	skipIfNoDocker(t)

	name := uniqueName(t)
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
	skipIfNoDocker(t)

	name := uniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()

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
	skipIfNoDocker(t)

	name := uniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()

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
	skipIfNoDocker(t)

	name := uniqueName(t)
	cleanupContainer(t, name)

	hostDir := t.TempDir()

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
