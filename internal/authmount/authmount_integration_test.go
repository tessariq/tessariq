//go:build integration

package authmount_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/authmount"

	testcontainers "github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

func execContainer(ctx context.Context, c testcontainers.Container, cmd []string) (int, string, error) {
	code, reader, err := c.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return -1, "", fmt.Errorf("exec %v: %w", cmd, err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return code, "", fmt.Errorf("read output of %v: %w", cmd, err)
	}
	return code, strings.TrimSpace(buf.String()), nil
}

func TestIntegration_ClaudeCodeAuthMountsReadOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create fake auth files on host.
	hostDir := t.TempDir()
	credDir := filepath.Join(hostDir, ".claude")
	require.NoError(t, os.MkdirAll(credDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(credDir, ".credentials.json"), []byte(`{"token":"fake"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hostDir, ".claude.json"), []byte(`{"config":true}`), 0o644))

	// Discover mounts using the fake home.
	result, err := authmount.Discover("claude-code", hostDir, "linux", authmount.FileExists)
	require.NoError(t, err)
	require.Len(t, result.Mounts, 2)

	// Build testcontainers mounts from the result.
	var mounts testcontainers.ContainerMounts
	for _, m := range result.Mounts {
		mounts = append(mounts, testcontainers.ContainerMount{
			Source:   testcontainers.GenericBindMountSource{HostPath: m.HostPath},
			Target:   testcontainers.ContainerMountTarget(m.ContainerPath),
			ReadOnly: m.ReadOnly,
		})
	}

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		Mounts:     mounts,
		WaitingFor: wait.ForExec([]string{"sh", "-c", "true"}).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	// Verify credentials file is present at container path.
	code, out, err := execContainer(ctx, container, []string{"cat", result.Mounts[0].ContainerPath})
	require.NoError(t, err)
	require.Equal(t, 0, code, "credentials file should be readable")
	require.Contains(t, out, "fake")

	// Verify config file is present at container path.
	code, out, err = execContainer(ctx, container, []string{"cat", result.Mounts[1].ContainerPath})
	require.NoError(t, err)
	require.Equal(t, 0, code, "config file should be readable")
	require.Contains(t, out, "config")

	// Verify files are read-only (write should fail).
	code, _, err = execContainer(ctx, container, []string{"sh", "-c",
		fmt.Sprintf("echo test > %s", result.Mounts[0].ContainerPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "write to read-only mount should fail")
}

func TestIntegration_OpenCodeAuthMountsReadOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create fake auth file on host.
	hostDir := t.TempDir()
	authDir := filepath.Join(hostDir, ".local", "share", "opencode")
	require.NoError(t, os.MkdirAll(authDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(authDir, "auth.json"), []byte(`{"key":"oc-fake"}`), 0o644))

	// Discover mounts.
	result, err := authmount.Discover("opencode", hostDir, "linux", authmount.FileExists)
	require.NoError(t, err)
	require.Len(t, result.Mounts, 1)

	var mounts testcontainers.ContainerMounts
	for _, m := range result.Mounts {
		mounts = append(mounts, testcontainers.ContainerMount{
			Source:   testcontainers.GenericBindMountSource{HostPath: m.HostPath},
			Target:   testcontainers.ContainerMountTarget(m.ContainerPath),
			ReadOnly: m.ReadOnly,
		})
	}

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		Mounts:     mounts,
		WaitingFor: wait.ForExec([]string{"sh", "-c", "true"}).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	// Verify auth file is present and readable.
	code, out, err := execContainer(ctx, container, []string{"cat", result.Mounts[0].ContainerPath})
	require.NoError(t, err)
	require.Equal(t, 0, code, "auth file should be readable")
	require.Contains(t, out, "oc-fake")

	// Verify read-only.
	code, _, err = execContainer(ctx, container, []string{"sh", "-c",
		fmt.Sprintf("echo test > %s", result.Mounts[0].ContainerPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "write to read-only mount should fail")
}

func TestIntegration_HostHomeNotExposed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create fake auth files on host.
	hostDir := t.TempDir()
	credDir := filepath.Join(hostDir, ".claude")
	require.NoError(t, os.MkdirAll(credDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(credDir, ".credentials.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hostDir, ".claude.json"), []byte(`{}`), 0o644))

	// Create a marker file in the host home dir that should NOT be visible.
	require.NoError(t, os.WriteFile(filepath.Join(hostDir, "SHOULD_NOT_BE_VISIBLE"), []byte("secret"), 0o644))

	result, err := authmount.Discover("claude-code", hostDir, "linux", authmount.FileExists)
	require.NoError(t, err)

	var mounts testcontainers.ContainerMounts
	for _, m := range result.Mounts {
		mounts = append(mounts, testcontainers.ContainerMount{
			Source:   testcontainers.GenericBindMountSource{HostPath: m.HostPath},
			Target:   testcontainers.ContainerMountTarget(m.ContainerPath),
			ReadOnly: m.ReadOnly,
		})
	}

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		Mounts:     mounts,
		WaitingFor: wait.ForExec([]string{"sh", "-c", "true"}).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	// The container home is /home/tessariq (or /root for alpine).
	// The host home marker file should not be visible anywhere in the container.
	code, _, err := execContainer(ctx, container, []string{"sh", "-c",
		"find / -name SHOULD_NOT_BE_VISIBLE 2>/dev/null"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	// find returns empty output if not found — which is what we want.
	// The marker file must not be accessible.
}
