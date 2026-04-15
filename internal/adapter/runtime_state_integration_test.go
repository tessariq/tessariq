//go:build integration

package adapter_test

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
	"github.com/tessariq/tessariq/internal/adapter"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/container"

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
		return code, "", fmt.Errorf("read output: %w", err)
	}
	return code, strings.TrimSpace(buf.String()), nil
}

// TestIntegration_ClaudeJsonSeedDoesNotPersistToHost is the regression test
// for BUG-050. It exercises the full seed-into-runtime flow:
//
//   - Fake Claude Code auth is placed on a host tempdir.
//   - authmount.Discover returns a spec for .claude.json with
//     SeedIntoRuntime=true.
//   - adapter.PrepareRuntimeState materializes a disposable scratch copy.
//   - The container mounts the effective (scratch-substituted) specs.
//   - In-container writes to ~/.claude.json mutate the scratch file,
//     which the test verifies is reflected inside the container.
//   - The HOST auth source must be byte-identical to its original content,
//     proving container-to-host persistence is not possible.
func TestIntegration_ClaudeJsonSeedDoesNotPersistToHost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	hostDir := t.TempDir()
	credDir := filepath.Join(hostDir, ".claude")
	require.NoError(t, os.MkdirAll(credDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(credDir, ".credentials.json"), []byte(`{"token":"fake"}`), 0o644))

	hostClaudeJSON := filepath.Join(hostDir, ".claude.json")
	originalContent := []byte(`{"numStartups":1,"features":{"initial":true}}`)
	require.NoError(t, os.WriteFile(hostClaudeJSON, originalContent, 0o600))

	result, err := authmount.Discover("claude-code", hostDir, "linux", authmount.FileExists)
	require.NoError(t, err)
	require.NoError(t, authmount.ValidateContract(result.Mounts),
		"Discover must return only read-only specs; writability is via SeedIntoRuntime")

	// At least one spec in the claude-code mount list must be seed-into-runtime.
	var seedFound bool
	for _, m := range result.Mounts {
		if m.SeedIntoRuntime {
			seedFound = true
			break
		}
	}
	require.True(t, seedFound, "claude-code must include a seed-into-runtime spec for .claude.json")

	scratchRoot := filepath.Join(t.TempDir(), "runtime-state", "run-abc")
	rs, err := adapter.PrepareAndHardenRuntimeState(ctx, scratchRoot, result.Mounts, container.RuntimeIdentity{UID: 1000, GID: 1000})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rs.Cleanup() })

	// Effective mounts must not contain any SeedIntoRuntime spec (they're
	// consumed by the transform) and must not expose the host auth file
	// directly for any spec originally marked as seed.
	for _, m := range rs.EffectiveMounts {
		require.False(t, m.SeedIntoRuntime, "effective mounts must not have SeedIntoRuntime set")
		require.NotEqual(t, hostClaudeJSON, m.HostPath,
			"host .claude.json must not be bound directly")
	}

	var mounts testcontainers.ContainerMounts
	for _, m := range rs.EffectiveMounts {
		mounts = append(mounts, testcontainers.ContainerMount{
			Source:   testcontainers.GenericBindMountSource{HostPath: m.HostPath},
			Target:   testcontainers.ContainerMountTarget(m.ContainerPath),
			ReadOnly: m.ReadOnly,
		})
	}

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		User:       "1000:1000",
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

	// Initial content inside the container reflects the seeded host content.
	code, out, err := execContainer(ctx, container, []string{"cat", filepath.Join(authmount.ContainerHome, ".claude.json")})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Contains(t, out, "initial")

	// In-container mutation of ~/.claude.json MUST succeed (the scratch is RW).
	mutatedContent := `{"numStartups":42,"features":{"mutated_from_container":true}}`
	code, _, err = execContainer(ctx, container, []string{"sh", "-c",
		fmt.Sprintf("echo '%s' > %s", mutatedContent, filepath.Join(authmount.ContainerHome, ".claude.json"))})
	require.NoError(t, err)
	require.Equal(t, 0, code, "in-container write to .claude.json must succeed against the scratch file")

	// Re-read from inside the container to confirm the mutation stuck to the
	// scratch file.
	code, out, err = execContainer(ctx, container, []string{"cat", filepath.Join(authmount.ContainerHome, ".claude.json")})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Contains(t, out, "mutated_from_container")

	// The HOST file MUST be unchanged. This is the BUG-050 regression guard.
	got, err := os.ReadFile(hostClaudeJSON)
	require.NoError(t, err)
	require.Equal(t, originalContent, got,
		"host .claude.json must not be mutated by in-container writes (BUG-050 regression guard)")

	// Credentials mount is always RO and must reject in-container writes.
	code, _, err = execContainer(ctx, container, []string{"sh", "-c",
		fmt.Sprintf("echo evil > %s", filepath.Join(authmount.ContainerHome, ".claude", ".credentials.json"))})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "credentials file must remain read-only")
}

// TestIntegration_OpenCodeAuthCannotPersistToHost confirms that the OpenCode
// auth path (which has no seed-into-runtime requirement today) stays RO, and
// that an in-container write attempt fails before it can touch the host.
func TestIntegration_OpenCodeAuthCannotPersistToHost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	hostDir := t.TempDir()
	authDir := filepath.Join(hostDir, ".local", "share", "opencode")
	require.NoError(t, os.MkdirAll(authDir, 0o755))

	hostAuth := filepath.Join(authDir, "auth.json")
	originalContent := []byte(`{"key":"oc-original"}`)
	require.NoError(t, os.WriteFile(hostAuth, originalContent, 0o600))

	result, err := authmount.Discover("opencode", hostDir, "linux", authmount.FileExists)
	require.NoError(t, err)
	require.NoError(t, authmount.ValidateContract(result.Mounts))

	rs, err := adapter.PrepareRuntimeState(filepath.Join(t.TempDir(), "runtime-state"), result.Mounts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rs.Cleanup() })

	// OpenCode has no seed specs today; effective mounts equal discovered mounts.
	require.Equal(t, result.Mounts, rs.EffectiveMounts)

	var mounts testcontainers.ContainerMounts
	for _, m := range rs.EffectiveMounts {
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

	// Write attempt inside container must fail.
	code, _, err := execContainer(ctx, container, []string{"sh", "-c",
		fmt.Sprintf("echo evil > %s", rs.EffectiveMounts[0].ContainerPath)})
	require.NoError(t, err)
	require.NotEqual(t, 0, code, "opencode auth.json mount must remain read-only")

	// Host file unchanged.
	got, err := os.ReadFile(hostAuth)
	require.NoError(t, err)
	require.Equal(t, originalContent, got, "host opencode auth.json must not be mutated")
}
