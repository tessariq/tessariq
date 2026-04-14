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

	// Credentials mount is RO and not seeded.
	require.Equal(t, credPath, result.Mounts[0].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".claude", ".credentials.json"), result.Mounts[0].ContainerPath)
	require.True(t, result.Mounts[0].ReadOnly)
	require.False(t, result.Mounts[0].SeedIntoRuntime)

	// Config mount is RO from the host and marked as needing a disposable
	// per-run runtime-state copy — the host file MUST NOT be bound writable.
	require.Equal(t, configPath, result.Mounts[1].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".claude.json"), result.Mounts[1].ContainerPath)
	require.True(t, result.Mounts[1].ReadOnly, ".claude.json host bind must be read-only")
	require.True(t, result.Mounts[1].SeedIntoRuntime, ".claude.json needs a disposable runtime-state copy")
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
				require.True(t, m.ReadOnly, "mount %s must be read-only; writability is expressed via SeedIntoRuntime", m.ContainerPath)
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

// --- DiscoverConfigDirs tests ---

func mockDirCheck(existing map[string]bool) func(string) bool {
	return func(path string) bool {
		return existing[path]
	}
}

func TestDiscoverConfigDirs_ClaudeCode(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".claude")

	tests := []struct {
		name       string
		exists     map[string]bool
		readable   map[string]bool
		wantStatus string
		wantCount  int
	}{
		{
			name:       "config dir present and readable",
			exists:     map[string]bool{configDir: true},
			readable:   map[string]bool{configDir: true},
			wantStatus: "mounted",
			wantCount:  1,
		},
		{
			name:       "config dir missing",
			exists:     map[string]bool{},
			readable:   map[string]bool{},
			wantStatus: "missing_optional",
			wantCount:  0,
		},
		{
			name:       "config dir exists but unreadable",
			exists:     map[string]bool{configDir: true},
			readable:   map[string]bool{},
			wantStatus: "unreadable_optional",
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := DiscoverConfigDirs("claude-code", home,
				mockDirCheck(tt.exists), mockDirCheck(tt.readable))

			require.NoError(t, err)
			require.Equal(t, "claude-code", result.Agent)
			require.Equal(t, tt.wantStatus, result.Status)
			require.Len(t, result.Mounts, tt.wantCount)
		})
	}
}

func TestDiscoverConfigDirs_ClaudeCode_MountDetails(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".claude")

	result, err := DiscoverConfigDirs("claude-code", home,
		mockDirCheck(map[string]bool{configDir: true}),
		mockDirCheck(map[string]bool{configDir: true}))

	require.NoError(t, err)
	require.Len(t, result.Mounts, 1)
	require.Equal(t, configDir, result.Mounts[0].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".claude"), result.Mounts[0].ContainerPath)
	require.True(t, result.Mounts[0].ReadOnly)
}

func TestDiscoverConfigDirs_ClaudeCode_EnvVars(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".claude")

	result, err := DiscoverConfigDirs("claude-code", home,
		mockDirCheck(map[string]bool{configDir: true}),
		mockDirCheck(map[string]bool{configDir: true}))

	require.NoError(t, err)
	require.Equal(t, filepath.Join(ContainerHome, ".claude"), result.EnvVars["CLAUDE_CONFIG_DIR"])
}

func TestDiscoverConfigDirs_ClaudeCode_MissingNoEnvVars(t *testing.T) {
	t.Parallel()

	result, err := DiscoverConfigDirs("claude-code", "/home/user",
		mockDirCheck(map[string]bool{}), mockDirCheck(map[string]bool{}))

	require.NoError(t, err)
	require.Empty(t, result.EnvVars)
}

func TestDiscoverConfigDirs_OpenCode(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".config", "opencode")

	tests := []struct {
		name       string
		exists     map[string]bool
		readable   map[string]bool
		wantStatus string
		wantCount  int
	}{
		{
			name:       "config dir present and readable",
			exists:     map[string]bool{configDir: true},
			readable:   map[string]bool{configDir: true},
			wantStatus: "mounted",
			wantCount:  1,
		},
		{
			name:       "config dir missing",
			exists:     map[string]bool{},
			readable:   map[string]bool{},
			wantStatus: "missing_optional",
			wantCount:  0,
		},
		{
			name:       "config dir exists but unreadable",
			exists:     map[string]bool{configDir: true},
			readable:   map[string]bool{},
			wantStatus: "unreadable_optional",
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := DiscoverConfigDirs("opencode", home,
				mockDirCheck(tt.exists), mockDirCheck(tt.readable))

			require.NoError(t, err)
			require.Equal(t, "opencode", result.Agent)
			require.Equal(t, tt.wantStatus, result.Status)
			require.Len(t, result.Mounts, tt.wantCount)
		})
	}
}

func TestDiscoverConfigDirs_OpenCode_MountDetails(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".config", "opencode")

	result, err := DiscoverConfigDirs("opencode", home,
		mockDirCheck(map[string]bool{configDir: true}),
		mockDirCheck(map[string]bool{configDir: true}))

	require.NoError(t, err)
	require.Len(t, result.Mounts, 1)
	require.Equal(t, configDir, result.Mounts[0].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".config", "opencode"), result.Mounts[0].ContainerPath)
	require.True(t, result.Mounts[0].ReadOnly)
}

func TestDiscoverConfigDirs_OpenCode_NoEnvVars(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".config", "opencode")

	result, err := DiscoverConfigDirs("opencode", home,
		mockDirCheck(map[string]bool{configDir: true}),
		mockDirCheck(map[string]bool{configDir: true}))

	require.NoError(t, err)
	require.Empty(t, result.EnvVars)
}

func TestDiscoverState_OpenCode(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	modelPath := filepath.Join(home, ".local", "state", "opencode", "model.json")

	tests := []struct {
		name       string
		existing   map[string]bool
		wantStatus string
		wantCount  int
	}{
		{
			name:       "model.json present",
			existing:   map[string]bool{modelPath: true},
			wantStatus: "mounted",
			wantCount:  1,
		},
		{
			name:       "model.json absent",
			existing:   map[string]bool{},
			wantStatus: "missing_optional",
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := DiscoverState("opencode", home, mockFileExists(tt.existing))
			require.NoError(t, err)
			require.Equal(t, "opencode", result.Agent)
			require.Equal(t, tt.wantStatus, result.Status)
			require.Len(t, result.Mounts, tt.wantCount)
		})
	}
}

func TestDiscoverState_OpenCode_MountDetails(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	modelPath := filepath.Join(home, ".local", "state", "opencode", "model.json")

	result, err := DiscoverState("opencode", home, mockFileExists(map[string]bool{
		modelPath: true,
	}))
	require.NoError(t, err)
	require.Len(t, result.Mounts, 1)

	require.Equal(t, modelPath, result.Mounts[0].HostPath)
	require.Equal(t, filepath.Join(ContainerHome, ".local", "state", "opencode", "model.json"), result.Mounts[0].ContainerPath)
	require.True(t, result.Mounts[0].ReadOnly)
}

func TestDiscoverState_ClaudeCode(t *testing.T) {
	t.Parallel()

	result, err := DiscoverState("claude-code", "/home/user", mockFileExists(nil))
	require.NoError(t, err)
	require.Equal(t, "claude-code", result.Agent)
	require.Equal(t, "missing_optional", result.Status)
	require.Empty(t, result.Mounts)
}

func TestDiscoverState_UnknownAgent(t *testing.T) {
	t.Parallel()

	_, err := DiscoverState("unknown-agent", "/home/user", func(string) bool { return true })
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown-agent")
}

func TestDiscoverConfigDirs_UnknownAgent(t *testing.T) {
	t.Parallel()

	_, err := DiscoverConfigDirs("unknown-agent", "/home/user",
		func(string) bool { return true }, func(string) bool { return true })
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown-agent")
}

func TestDiscoverConfigDirs_AllMountsReadOnly(t *testing.T) {
	t.Parallel()

	home := "/home/user"

	agents := []struct {
		agent     string
		configDir string
	}{
		{agent: "claude-code", configDir: filepath.Join(home, ".claude")},
		{agent: "opencode", configDir: filepath.Join(home, ".config", "opencode")},
	}

	for _, tt := range agents {
		t.Run(tt.agent, func(t *testing.T) {
			t.Parallel()

			result, err := DiscoverConfigDirs(tt.agent, home,
				mockDirCheck(map[string]bool{tt.configDir: true}),
				mockDirCheck(map[string]bool{tt.configDir: true}))
			require.NoError(t, err)

			for _, m := range result.Mounts {
				require.True(t, m.ReadOnly, "mount %s must be read-only", m.ContainerPath)
			}
		})
	}
}

func TestDiscoverConfigDirs_PathsAreAbsolute(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".claude")

	result, err := DiscoverConfigDirs("claude-code", home,
		mockDirCheck(map[string]bool{configDir: true}),
		mockDirCheck(map[string]bool{configDir: true}))
	require.NoError(t, err)

	for _, m := range result.Mounts {
		require.True(t, filepath.IsAbs(m.HostPath), "host path %s must be absolute", m.HostPath)
		require.True(t, filepath.IsAbs(m.ContainerPath), "container path %s must be absolute", m.ContainerPath)
	}
}

func TestDiscoverConfigDirs_NoHostHomeExposure(t *testing.T) {
	t.Parallel()

	home := "/home/user"
	configDir := filepath.Join(home, ".claude")

	result, err := DiscoverConfigDirs("claude-code", home,
		mockDirCheck(map[string]bool{configDir: true}),
		mockDirCheck(map[string]bool{configDir: true}))
	require.NoError(t, err)

	for _, m := range result.Mounts {
		require.NotEqual(t, home, m.ContainerPath,
			"container path must not be the host HOME directory")
	}
}

// TestDiscover_SupportedAgentsReadOnlyInvariant iterates every Discover*
// entry point for every supported agent and asserts that no returned
// MountSpec is writable from the host. Writability is expressed exclusively
// via SeedIntoRuntime, which causes the caller to substitute a disposable
// per-run scratch file.
func TestDiscover_SupportedAgentsReadOnlyInvariant(t *testing.T) {
	t.Parallel()

	home := "/home/user"

	type agentFixture struct {
		agent      string
		authFiles  map[string]bool
		stateFiles map[string]bool
		configDirs map[string]bool
	}

	fixtures := []agentFixture{
		{
			agent: "claude-code",
			authFiles: map[string]bool{
				filepath.Join(home, ".claude", ".credentials.json"): true,
				filepath.Join(home, ".claude.json"):                 true,
			},
			stateFiles: map[string]bool{},
			configDirs: map[string]bool{filepath.Join(home, ".claude"): true},
		},
		{
			agent: "opencode",
			authFiles: map[string]bool{
				filepath.Join(home, ".local", "share", "opencode", "auth.json"): true,
			},
			stateFiles: map[string]bool{
				filepath.Join(home, ".local", "state", "opencode", "model.json"): true,
			},
			configDirs: map[string]bool{filepath.Join(home, ".config", "opencode"): true},
		},
	}

	for _, fx := range fixtures {
		t.Run(fx.agent, func(t *testing.T) {
			t.Parallel()

			auth, err := Discover(fx.agent, home, "linux", mockFileExists(fx.authFiles))
			require.NoError(t, err)
			for _, m := range auth.Mounts {
				require.True(t, m.ReadOnly,
					"Discover(%s) returned writable spec for %s", fx.agent, m.ContainerPath)
			}

			state, err := DiscoverState(fx.agent, home, mockFileExists(fx.stateFiles))
			require.NoError(t, err)
			for _, m := range state.Mounts {
				require.True(t, m.ReadOnly,
					"DiscoverState(%s) returned writable spec for %s", fx.agent, m.ContainerPath)
			}

			cfg, err := DiscoverConfigDirs(fx.agent, home,
				mockDirCheck(fx.configDirs), mockDirCheck(fx.configDirs))
			require.NoError(t, err)
			for _, m := range cfg.Mounts {
				require.True(t, m.ReadOnly,
					"DiscoverConfigDirs(%s) returned writable spec for %s", fx.agent, m.ContainerPath)
				require.False(t, m.SeedIntoRuntime,
					"config dirs must never be seeded into runtime")
			}
		})
	}
}

func TestValidateContract_AcceptsReadOnlySpecs(t *testing.T) {
	t.Parallel()

	specs := []MountSpec{
		{HostPath: "/h/a", ContainerPath: "/c/a", ReadOnly: true},
		{HostPath: "/h/b", ContainerPath: "/c/b", ReadOnly: true, SeedIntoRuntime: true},
	}
	require.NoError(t, ValidateContract(specs))
}

func TestValidateContract_RejectsWritableSpec(t *testing.T) {
	t.Parallel()

	specs := []MountSpec{
		{HostPath: "/h/a", ContainerPath: "/c/a", ReadOnly: true},
		{HostPath: "/h/b", ContainerPath: "/c/b", ReadOnly: false},
	}
	err := ValidateContract(specs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/h/b")
	require.Contains(t, err.Error(), "writable")
}

func TestValidateContract_EmptyIsValid(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateContract(nil))
	require.NoError(t, ValidateContract([]MountSpec{}))
}
