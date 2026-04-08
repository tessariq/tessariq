//go:build integration

package container_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/container"
)

func TestRunInitContainer_SuccessfulCommand(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	ctx := context.Background()

	result := container.RunInitContainer(ctx, container.InitConfig{
		Image:         "alpine:latest",
		Command:       []string{"sh", "-c", "mkdir -p /cache/bin && echo 'hello' > /cache/bin/test-agent"},
		VersionCmd:    []string{"sh", "-c", "echo v1.0.0"},
		CacheHostPath: cacheDir,
		AgentName:     "test-agent",
		Timeout:       60 * time.Second,
	})

	require.True(t, result.Success, "init container should succeed")
	require.Greater(t, result.ElapsedMs, int64(0), "elapsed time must be positive")
	require.Empty(t, result.Error)

	// Verify file was written to cache.
	content, err := os.ReadFile(filepath.Join(cacheDir, "bin", "test-agent"))
	require.NoError(t, err)
	require.Equal(t, "hello\n", string(content))
}

func TestRunInitContainer_CachePersistence(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	ctx := context.Background()

	// First run: write a file.
	result1 := container.RunInitContainer(ctx, container.InitConfig{
		Image:         "alpine:latest",
		Command:       []string{"sh", "-c", "mkdir -p /cache/bin && echo 'v1' > /cache/bin/agent"},
		VersionCmd:    []string{"sh", "-c", "echo v1.0.0"},
		CacheHostPath: cacheDir,
		AgentName:     "test-agent",
		Timeout:       60 * time.Second,
	})
	require.True(t, result1.Success)

	// Second run: verify file exists and overwrite.
	result2 := container.RunInitContainer(ctx, container.InitConfig{
		Image:         "alpine:latest",
		Command:       []string{"sh", "-c", "cat /cache/bin/agent && echo 'v2' > /cache/bin/agent"},
		VersionCmd:    []string{"sh", "-c", "echo v2.0.0"},
		CacheHostPath: cacheDir,
		AgentName:     "test-agent",
		Timeout:       60 * time.Second,
	})
	require.True(t, result2.Success, "second run error: %s", result2.Error)

	// Verify second write persisted.
	content, err := os.ReadFile(filepath.Join(cacheDir, "bin", "agent"))
	require.NoError(t, err)
	require.Equal(t, "v2\n", string(content))
}

func TestRunInitContainer_Timeout(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	ctx := context.Background()

	result := container.RunInitContainer(ctx, container.InitConfig{
		Image:         "alpine:latest",
		Command:       []string{"sleep", "300"},
		VersionCmd:    []string{"sh", "-c", "echo v1.0.0"},
		CacheHostPath: cacheDir,
		AgentName:     "test-agent",
		Timeout:       2 * time.Second,
	})

	require.False(t, result.Success, "timed-out init container should report failure")
	require.Contains(t, result.Error, "timed out")
}

func TestRunInitContainer_NonZeroExit(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	ctx := context.Background()

	result := container.RunInitContainer(ctx, container.InitConfig{
		Image:         "alpine:latest",
		Command:       []string{"sh", "-c", "echo 'install failed' >&2; exit 1"},
		VersionCmd:    []string{"sh", "-c", "echo v1.0.0"},
		CacheHostPath: cacheDir,
		AgentName:     "test-agent",
		Timeout:       60 * time.Second,
	})

	require.False(t, result.Success, "non-zero exit should report failure")
	require.NotEmpty(t, result.Error)
}

func TestRunInitContainer_VersionProbe(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	ctx := context.Background()

	result := container.RunInitContainer(ctx, container.InitConfig{
		Image:         "alpine:latest",
		Command:       []string{"sh", "-c", "mkdir -p /cache/bin && printf '#!/bin/sh\\necho v2.0.0' > /cache/bin/my-agent && chmod +x /cache/bin/my-agent"},
		VersionCmd:    []string{"sh", "-c", "echo v1.0.0"},
		CacheHostPath: cacheDir,
		AgentName:     "test-agent",
		Timeout:       60 * time.Second,
	})

	require.True(t, result.Success)
	require.Equal(t, "v1.0.0", result.BakedVersion)
}
