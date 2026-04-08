package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildInitRunArgs_FullCommand(t *testing.T) {
	t.Parallel()

	cfg := InitConfig{
		Image:         "ghcr.io/tessariq/claude-code:v0.1.0",
		Command:       []string{"npm", "install", "--global", "--prefix", "/cache", "@anthropic-ai/claude-code@latest"},
		CacheHostPath: "/home/user/.tessariq/agent-cache/claude-code",
		AgentName:     "claude-code",
		Timeout:       120 * time.Second,
	}

	args := buildInitRunArgs(cfg)

	expected := []string{
		"run", "--rm",
		"--cap-drop", "ALL",
		"--cap-add", "DAC_OVERRIDE",
		"--cap-add", "CHOWN",
		"--cap-add", "FOWNER",
		"--security-opt", "no-new-privileges",
		"--user", "root",
		"--entrypoint", "",
		"-v", "/home/user/.tessariq/agent-cache/claude-code:/cache",
		"ghcr.io/tessariq/claude-code:v0.1.0",
		"npm", "install", "--global", "--prefix", "/cache", "@anthropic-ai/claude-code@latest",
	}
	require.Equal(t, expected, args)
}

func TestBuildInitRunArgs_SecurityFlags(t *testing.T) {
	t.Parallel()

	cfg := InitConfig{
		Image:         "img:latest",
		Command:       []string{"echo", "hi"},
		CacheHostPath: "/cache",
	}

	args := buildInitRunArgs(cfg)

	// Init container drops all caps then adds back only the two it needs.
	require.Contains(t, args, "--cap-drop")
	require.Contains(t, args, "ALL")
	require.Contains(t, args, "--cap-add")
	require.Contains(t, args, "DAC_OVERRIDE")
	require.Contains(t, args, "CHOWN")
	require.Contains(t, args, "FOWNER")
	require.Contains(t, args, "--security-opt")
	require.Contains(t, args, "no-new-privileges")
}

func TestBuildInitRunArgs_RootUser(t *testing.T) {
	t.Parallel()

	cfg := InitConfig{
		Image:         "img:latest",
		Command:       []string{"echo", "hi"},
		CacheHostPath: "/cache",
	}

	args := buildInitRunArgs(cfg)

	require.Contains(t, args, "--user")
	require.Contains(t, args, "root")
}

func TestBuildInitRunArgs_CacheMount(t *testing.T) {
	t.Parallel()

	cfg := InitConfig{
		Image:         "img:latest",
		Command:       []string{"echo", "hi"},
		CacheHostPath: "/home/user/.tessariq/agent-cache/opencode",
	}

	args := buildInitRunArgs(cfg)

	require.Contains(t, args, "-v")
	require.Contains(t, args, "/home/user/.tessariq/agent-cache/opencode:/cache")
}

func TestBuildInitRunArgs_NoWorkDir(t *testing.T) {
	t.Parallel()

	cfg := InitConfig{
		Image:         "img:latest",
		Command:       []string{"echo", "hi"},
		CacheHostPath: "/cache",
	}

	args := buildInitRunArgs(cfg)

	require.NotContains(t, args, "--workdir")
}

func TestBuildInitRunArgs_EntryPointOverride(t *testing.T) {
	t.Parallel()

	cfg := InitConfig{
		Image:         "img:latest",
		Command:       []string{"echo", "hi"},
		CacheHostPath: "/cache",
	}

	args := buildInitRunArgs(cfg)

	require.Contains(t, args, "--entrypoint")
	// --entrypoint "" must appear to override any image entrypoint
	entrypointIdx := -1
	for i, a := range args {
		if a == "--entrypoint" {
			entrypointIdx = i
			break
		}
	}
	require.Greater(t, entrypointIdx, -1)
	require.Equal(t, "", args[entrypointIdx+1])
}

func TestBuildInitRunArgs_RemovesContainerOnExit(t *testing.T) {
	t.Parallel()

	cfg := InitConfig{
		Image:         "img:latest",
		Command:       []string{"echo"},
		CacheHostPath: "/cache",
	}

	args := buildInitRunArgs(cfg)

	require.Contains(t, args, "--rm")
}
