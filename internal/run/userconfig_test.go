package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserConfigPath_XDGSet_DirExists(t *testing.T) {
	t.Parallel()

	dirExists := func(string) bool { return true }
	path := UserConfigPath("/custom/xdg", "/home/user", dirExists)
	require.Equal(t, filepath.Join("/custom/xdg", "tessariq", "config.yaml"), path)
}

func TestUserConfigPath_XDGSet_DirMissing(t *testing.T) {
	t.Parallel()

	dirExists := func(string) bool { return false }
	path := UserConfigPath("/custom/xdg", "/home/user", dirExists)
	require.Empty(t, path)
}

func TestUserConfigPath_XDGUnset_DefaultExists(t *testing.T) {
	t.Parallel()

	dirExists := func(p string) bool {
		return p == filepath.Join("/home/user", ".config", "tessariq")
	}
	path := UserConfigPath("", "/home/user", dirExists)
	require.Equal(t, filepath.Join("/home/user", ".config", "tessariq", "config.yaml"), path)
}

func TestUserConfigPath_XDGUnset_DefaultMissing(t *testing.T) {
	t.Parallel()

	dirExists := func(string) bool { return false }
	path := UserConfigPath("", "/home/user", dirExists)
	require.Empty(t, path)
}

func TestLoadUserConfig_EmptyPath(t *testing.T) {
	t.Parallel()

	cfg, err := LoadUserConfig("", nil)
	require.NoError(t, err)
	require.Nil(t, cfg)
}

func TestLoadUserConfig_FileNotFound(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	cfg, err := LoadUserConfig("/some/config.yaml", readFile)
	require.NoError(t, err)
	require.Nil(t, cfg)
}

func TestLoadUserConfig_PermissionDenied(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return nil, os.ErrPermission
	}
	_, err := LoadUserConfig("/some/config.yaml", readFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unreadable")
}

func TestLoadUserConfig_MalformedYAML(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return []byte("egress_allow:\n  - [invalid\n  broken"), nil
	}
	_, err := LoadUserConfig("/some/config.yaml", readFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "malformed")
	require.Contains(t, err.Error(), "YAML syntax")
}

func TestLoadUserConfig_ValidConfig(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return []byte("egress_allow:\n  - api.example.com:443\n  - cdn.example.com:443\n"), nil
	}
	cfg, err := LoadUserConfig("/some/config.yaml", readFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, []string{"api.example.com:443", "cdn.example.com:443"}, cfg.EgressAllow)
}

func TestLoadUserConfig_UnknownKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "camelCase_typo",
			input: "egressAllow:\n  - api.example.com:443\n",
		},
		{
			name:  "misspelled_key",
			input: "egress_alow:\n  - api.example.com:443\n",
		},
		{
			name:  "unknown_extra_key",
			input: "egress_allow:\n  - api.example.com:443\nfuture_key: some_value\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			readFile := func(string) ([]byte, error) {
				return []byte(tt.input), nil
			}
			_, err := LoadUserConfig("/some/config.yaml", readFile)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown field")
			require.Contains(t, err.Error(), "/some/config.yaml")
		})
	}
}

func TestLoadUserConfig_EmptyFile(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return []byte(""), nil
	}
	cfg, err := LoadUserConfig("/some/config.yaml", readFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Empty(t, cfg.EgressAllow)
}

func TestLoadUserConfig_EmptyAllowlist(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return []byte("egress_allow: []\n"), nil
	}
	cfg, err := LoadUserConfig("/some/config.yaml", readFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Empty(t, cfg.EgressAllow)
}

func TestLoadUserConfig_ReadError(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		return nil, errors.New("disk failure")
	}
	_, err := LoadUserConfig("/some/config.yaml", readFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read config file")
}
