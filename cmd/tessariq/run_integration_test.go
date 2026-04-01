//go:build integration

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/run"
)

func TestIntegration_ResolveAllowlistCore_MissingAuthReturnsAuthMissingError(t *testing.T) {
	t.Parallel()

	// Use a temp dir as "home" — no auth.json exists on disk.
	homeDir := t.TempDir()

	deps := resolveAllowlistDeps{
		xdgConfigHome: "",
		dirExists: func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && info.IsDir()
		},
		readFile: os.ReadFile,
	}

	cfg := run.Config{
		Agent: "opencode",
	}

	_, err := resolveAllowlistCore(cfg, homeDir, "proxy", deps)
	require.Error(t, err)

	var authMissing *authmount.AuthMissingError
	require.ErrorAs(t, err, &authMissing)
	require.Equal(t, "opencode", authMissing.Agent)
	require.Contains(t, err.Error(), "authenticate opencode locally first")
}

func TestIntegration_ResolveAllowlistCore_ValidAuthUnchanged(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()

	// Create a valid auth.json with provider info.
	authDir := homeDir + "/.local/share/opencode"
	require.NoError(t, os.MkdirAll(authDir, 0o755))
	require.NoError(t, os.WriteFile(authDir+"/auth.json",
		[]byte(`{"token":"fake","provider":"https://api.example.com"}`), 0o644))

	deps := resolveAllowlistDeps{
		xdgConfigHome: "",
		dirExists: func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && info.IsDir()
		},
		readFile: os.ReadFile,
	}

	cfg := run.Config{
		Agent: "opencode",
	}

	result, err := resolveAllowlistCore(cfg, homeDir, "proxy", deps)
	require.NoError(t, err)
	require.Equal(t, "built_in", result.Source)
}
