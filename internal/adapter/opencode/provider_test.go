package opencode

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveProvider_ConfigWithProviderURL(t *testing.T) {
	t.Parallel()
	info, err := ResolveProvider(
		[]byte(`{"key":"oc-fake"}`),
		[]byte(`{"provider":"https://api.anthropic.com/v1"}`),
	)
	require.NoError(t, err)
	require.Equal(t, "api.anthropic.com", info.Host)
	require.False(t, info.IsOpenCodeHosted)
}

func TestResolveProvider_ConfigWithBareHostname(t *testing.T) {
	t.Parallel()
	info, err := ResolveProvider(
		[]byte(`{"key":"oc-fake"}`),
		[]byte(`{"provider":"api.example.com"}`),
	)
	require.NoError(t, err)
	require.Equal(t, "api.example.com", info.Host)
	require.False(t, info.IsOpenCodeHosted)
}

func TestResolveProvider_NoConfig_AuthWithProvider(t *testing.T) {
	t.Parallel()
	info, err := ResolveProvider(
		[]byte(`{"key":"oc-fake","provider":"https://api.example.com"}`),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, "api.example.com", info.Host)
	require.False(t, info.IsOpenCodeHosted)
}

func TestResolveProvider_NoConfig_AuthWithBaseURL(t *testing.T) {
	t.Parallel()
	info, err := ResolveProvider(
		[]byte(`{"key":"oc-fake","base_url":"https://models.example.com/api"}`),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, "models.example.com", info.Host)
	require.False(t, info.IsOpenCodeHosted)
}

func TestResolveProvider_NeitherHasProvider(t *testing.T) {
	t.Parallel()
	_, err := ResolveProvider(
		[]byte(`{"key":"oc-fake"}`),
		nil,
	)
	require.Error(t, err)
	var unresolvable *ProviderUnresolvableError
	require.ErrorAs(t, err, &unresolvable)
}

func TestResolveProvider_ConfigTakesPrecedenceOverAuth(t *testing.T) {
	t.Parallel()
	info, err := ResolveProvider(
		[]byte(`{"key":"oc-fake","provider":"https://auth-provider.com"}`),
		[]byte(`{"provider":"https://config-provider.com"}`),
	)
	require.NoError(t, err)
	require.Equal(t, "config-provider.com", info.Host)
}

func TestResolveProvider_OpenCodeHosted(t *testing.T) {
	t.Parallel()
	info, err := ResolveProvider(
		[]byte(`{"key":"oc-fake"}`),
		[]byte(`{"provider":"https://opencode.ai/api"}`),
	)
	require.NoError(t, err)
	require.Equal(t, "opencode.ai", info.Host)
	require.True(t, info.IsOpenCodeHosted)
}

func TestResolveProvider_OpenCodeSubdomainHosted(t *testing.T) {
	t.Parallel()
	info, err := ResolveProvider(
		[]byte(`{"key":"oc-fake"}`),
		[]byte(`{"provider":"https://api.opencode.ai"}`),
	)
	require.NoError(t, err)
	require.Equal(t, "api.opencode.ai", info.Host)
	require.True(t, info.IsOpenCodeHosted)
}

func TestResolveProvider_MalformedAuthJSON(t *testing.T) {
	t.Parallel()
	_, err := ResolveProvider([]byte(`{invalid`), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse auth")
}

func TestResolveProvider_MalformedConfigJSON(t *testing.T) {
	t.Parallel()
	_, err := ResolveProvider(
		[]byte(`{"key":"oc-fake"}`),
		[]byte(`{invalid`),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse config")
}

func TestResolveProvider_EmptyProviderField(t *testing.T) {
	t.Parallel()
	_, err := ResolveProvider(
		[]byte(`{"key":"oc-fake"}`),
		[]byte(`{"provider":""}`),
	)
	require.Error(t, err)
	var unresolvable *ProviderUnresolvableError
	require.ErrorAs(t, err, &unresolvable)
}

func TestResolveProvider_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		auth             string
		config           string
		wantHost         string
		wantOpenCodeHost bool
		wantErr          bool
	}{
		{
			name:     "config URL with path",
			auth:     `{"key":"k"}`,
			config:   `{"provider":"https://api.together.xyz/v1"}`,
			wantHost: "api.together.xyz",
		},
		{
			name:     "auth base_url only",
			auth:     `{"key":"k","base_url":"https://api.fireworks.ai"}`,
			wantHost: "api.fireworks.ai",
		},
		{
			name:             "opencode.ai bare hostname in config",
			auth:             `{"key":"k"}`,
			config:           `{"provider":"opencode.ai"}`,
			wantHost:         "opencode.ai",
			wantOpenCodeHost: true,
		},
		{
			name:    "no provider info at all",
			auth:    `{"key":"k"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var configData []byte
			if tt.config != "" {
				configData = []byte(tt.config)
			}
			info, err := ResolveProvider([]byte(tt.auth), configData)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantHost, info.Host)
			require.Equal(t, tt.wantOpenCodeHost, info.IsOpenCodeHosted)
		})
	}
}

func TestProviderUnresolvableError_Message(t *testing.T) {
	t.Parallel()
	err := &ProviderUnresolvableError{}
	require.Contains(t, err.Error(), "configure the provider")
	require.Contains(t, err.Error(), "--egress-allow")
}

func TestResolveProviderFromPaths_WithConfigDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	configDir := filepath.Join(dir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(authPath, []byte(`{"key":"k"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{"provider":"https://api.example.com"}`), 0o644))

	info, err := ResolveProviderFromPaths(authPath, configDir, os.ReadFile)
	require.NoError(t, err)
	require.Equal(t, "api.example.com", info.Host)
}

func TestResolveProviderFromPaths_NoConfigJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	configDir := filepath.Join(dir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(authPath, []byte(`{"key":"k","provider":"https://fallback.com"}`), 0o644))

	info, err := ResolveProviderFromPaths(authPath, configDir, os.ReadFile)
	require.NoError(t, err)
	require.Equal(t, "fallback.com", info.Host)
}

func TestResolveProviderFromPaths_EmptyConfigDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	require.NoError(t, os.WriteFile(authPath, []byte(`{"key":"k","base_url":"https://api.test.com"}`), 0o644))

	info, err := ResolveProviderFromPaths(authPath, "", os.ReadFile)
	require.NoError(t, err)
	require.Equal(t, "api.test.com", info.Host)
}

func TestResolveProviderFromPaths_AuthUnreadable(t *testing.T) {
	t.Parallel()
	readFile := func(string) ([]byte, error) {
		return nil, errors.New("permission denied")
	}
	_, err := ResolveProviderFromPaths("/fake/auth.json", "", readFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read auth")
}
