package authmount

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func mockFileExists(existing map[string]bool) func(string) bool {
	return func(path string) bool {
		return existing[path]
	}
}

func TestDiscover_ErrorTypes(t *testing.T) {
	t.Parallel()

	t.Run("AuthMissingError message includes agent name", func(t *testing.T) {
		t.Parallel()
		err := &AuthMissingError{Agent: "claude-code"}
		require.Contains(t, err.Error(), "claude-code")
		require.Contains(t, err.Error(), "authenticate")
	})

	t.Run("KeychainOnlyError message references credentials.json", func(t *testing.T) {
		t.Parallel()
		err := &KeychainOnlyError{}
		require.Contains(t, err.Error(), ".credentials.json")
		require.Contains(t, err.Error(), "file-backed setup")
	})

	t.Run("WritableAuthRequiredError message references read-only", func(t *testing.T) {
		t.Parallel()
		err := &WritableAuthRequiredError{Agent: "test-agent"}
		require.Contains(t, err.Error(), "read-only")
		require.Contains(t, err.Error(), "pre-authenticated")
	})
}

func TestDiscover_UnknownAgent(t *testing.T) {
	t.Parallel()

	_, err := Discover("unknown-agent", "/home/user", "linux", func(string) bool { return true })
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown-agent")
}

func TestDiscover_ClaudeCode(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	credPath := filepath.Join(home, ".claude", ".credentials.json")
	configPath := filepath.Join(home, ".claude.json")

	tests := []struct {
		name         string
		goos         string
		existing     map[string]bool
		wantMissing  bool
		wantKeychain bool
		wantCount    int
	}{
		{
			name:      "linux both present",
			goos:      "linux",
			existing:  map[string]bool{credPath: true, configPath: true},
			wantCount: 2,
		},
		{
			name:        "linux credentials missing",
			goos:        "linux",
			existing:    map[string]bool{configPath: true},
			wantMissing: true,
		},
		{
			name:        "linux config missing",
			goos:        "linux",
			existing:    map[string]bool{credPath: true},
			wantMissing: true,
		},
		{
			name:        "linux both missing",
			goos:        "linux",
			existing:    map[string]bool{},
			wantMissing: true,
		},
		{
			name:      "macos both present",
			goos:      "darwin",
			existing:  map[string]bool{credPath: true, configPath: true},
			wantCount: 2,
		},
		{
			name:         "macos credentials missing config present is keychain only",
			goos:         "darwin",
			existing:     map[string]bool{configPath: true},
			wantKeychain: true,
		},
		{
			name:        "macos both missing",
			goos:        "darwin",
			existing:    map[string]bool{},
			wantMissing: true,
		},
		{
			name:        "macos config missing credentials present",
			goos:        "darwin",
			existing:    map[string]bool{credPath: true},
			wantMissing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := Discover("claude-code", home, tt.goos, mockFileExists(tt.existing))

			if tt.wantMissing {
				var target *AuthMissingError
				require.ErrorAs(t, err, &target)
				return
			}
			if tt.wantKeychain {
				var target *KeychainOnlyError
				require.ErrorAs(t, err, &target)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "claude-code", result.Agent)
			require.Len(t, result.Mounts, tt.wantCount)
		})
	}
}

func TestDiscover_ClaudeCode_MountDetails(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	credPath := filepath.Join(home, ".claude", ".credentials.json")
	configPath := filepath.Join(home, ".claude.json")

	result, err := Discover("claude-code", home, "linux", mockFileExists(map[string]bool{
		credPath:   true,
		configPath: true,
	}))
	require.NoError(t, err)
	require.Len(t, result.Mounts, 2)

	// Credentials mount.
	require.Equal(t, credPath, result.Mounts[0].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".claude", ".credentials.json"), result.Mounts[0].ContainerPath)
	require.True(t, result.Mounts[0].ReadOnly)

	// Config mount.
	require.Equal(t, configPath, result.Mounts[1].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".claude.json"), result.Mounts[1].ContainerPath)
	require.True(t, result.Mounts[1].ReadOnly)
}

func TestDiscover_ClaudeCode_NoHostHomeExposure(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	credPath := filepath.Join(home, ".claude", ".credentials.json")
	configPath := filepath.Join(home, ".claude.json")

	result, err := Discover("claude-code", home, "linux", mockFileExists(map[string]bool{
		credPath:   true,
		configPath: true,
	}))
	require.NoError(t, err)

	for _, m := range result.Mounts {
		require.NotEqual(t, home, m.ContainerPath,
			"container path must not be the host HOME directory")
		require.NotEqual(t, home, m.HostPath[:len(home)]+"/",
			"host path must be a subpath, not HOME itself")
	}
}

func TestDiscover_OpenCode(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")

	tests := []struct {
		name        string
		goos        string
		existing    map[string]bool
		wantMissing bool
		wantCount   int
	}{
		{
			name:      "linux auth present",
			goos:      "linux",
			existing:  map[string]bool{authPath: true},
			wantCount: 1,
		},
		{
			name:        "linux auth missing",
			goos:        "linux",
			existing:    map[string]bool{},
			wantMissing: true,
		},
		{
			name:      "macos auth present",
			goos:      "darwin",
			existing:  map[string]bool{authPath: true},
			wantCount: 1,
		},
		{
			name:        "macos auth missing",
			goos:        "darwin",
			existing:    map[string]bool{},
			wantMissing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := Discover("opencode", home, tt.goos, mockFileExists(tt.existing))

			if tt.wantMissing {
				var target *AuthMissingError
				require.ErrorAs(t, err, &target)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "opencode", result.Agent)
			require.Len(t, result.Mounts, tt.wantCount)
		})
	}
}

func TestDiscover_OpenCode_MountDetails(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")

	result, err := Discover("opencode", home, "linux", mockFileExists(map[string]bool{
		authPath: true,
	}))
	require.NoError(t, err)
	require.Len(t, result.Mounts, 1)

	require.Equal(t, authPath, result.Mounts[0].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".local", "share", "opencode", "auth.json"), result.Mounts[0].ContainerPath)
	require.True(t, result.Mounts[0].ReadOnly)
}

func TestDiscover_AllMountsAreReadOnly(t *testing.T) {
	t.Parallel()

	home := "/home/user"

	agents := []struct {
		agent    string
		existing map[string]bool
	}{
		{
			agent: "claude-code",
			existing: map[string]bool{
				filepath.Join(home, ".claude", ".credentials.json"): true,
				filepath.Join(home, ".claude.json"):                 true,
			},
		},
		{
			agent: "opencode",
			existing: map[string]bool{
				filepath.Join(home, ".local", "share", "opencode", "auth.json"): true,
			},
		},
	}

	for _, tt := range agents {
		t.Run(tt.agent, func(t *testing.T) {
			t.Parallel()

			result, err := Discover(tt.agent, home, "linux", mockFileExists(tt.existing))
			require.NoError(t, err)

			for _, m := range result.Mounts {
				require.True(t, m.ReadOnly, "mount %s must be read-only", m.ContainerPath)
			}
		})
	}
}

func TestDiscover_HostPathsAreAbsolute(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	result, err := Discover("claude-code", home, "linux", mockFileExists(map[string]bool{
		filepath.Join(home, ".claude", ".credentials.json"): true,
		filepath.Join(home, ".claude.json"):                 true,
	}))
	require.NoError(t, err)

	for _, m := range result.Mounts {
		require.True(t, filepath.IsAbs(m.HostPath), "host path %s must be absolute", m.HostPath)
		require.True(t, filepath.IsAbs(m.ContainerPath), "container path %s must be absolute", m.ContainerPath)
	}
}
