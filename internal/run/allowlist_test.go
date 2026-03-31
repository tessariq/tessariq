package run

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDestination_HostPort(t *testing.T) {
	t.Parallel()

	host, port, err := ParseDestination("example.com:443")
	require.NoError(t, err)
	require.Equal(t, "example.com", host)
	require.Equal(t, 443, port)
}

func TestParseDestination_HostOnly(t *testing.T) {
	t.Parallel()

	host, port, err := ParseDestination("example.com")
	require.NoError(t, err)
	require.Equal(t, "example.com", host)
	require.Equal(t, 443, port)
}

func TestParseDestination_NonNumericPort(t *testing.T) {
	t.Parallel()

	_, _, err := ParseDestination("example.com:abc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-numeric port")
}

func TestParseDestination_EmptyHost(t *testing.T) {
	t.Parallel()

	_, _, err := ParseDestination(":443")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty host")
}

func TestParseDestination_PortZero(t *testing.T) {
	t.Parallel()

	_, _, err := ParseDestination("example.com:0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "port")
}

func TestParseDestination_PortTooHigh(t *testing.T) {
	t.Parallel()

	_, _, err := ParseDestination("example.com:99999")
	require.Error(t, err)
	require.Contains(t, err.Error(), "port")
}

func TestParseDestination_HostWithSpaces(t *testing.T) {
	t.Parallel()

	_, _, err := ParseDestination("bad host:443")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid host")
}

func TestParseDestination_EmptyString(t *testing.T) {
	t.Parallel()

	_, _, err := ParseDestination("")
	require.Error(t, err)
}

func TestParseDestination_PortOne_Valid(t *testing.T) {
	t.Parallel()

	_, port, err := ParseDestination("example.com:1")
	require.NoError(t, err)
	require.Equal(t, 1, port)
}

func TestParseDestination_Port65535_Valid(t *testing.T) {
	t.Parallel()

	_, port, err := ParseDestination("example.com:65535")
	require.NoError(t, err)
	require.Equal(t, 65535, port)
}

func TestParseDestination_CustomPort(t *testing.T) {
	t.Parallel()

	host, port, err := ParseDestination("registry.example.com:8443")
	require.NoError(t, err)
	require.Equal(t, "registry.example.com", host)
	require.Equal(t, 8443, port)
}

// --- ResolveAllowlist tests ---

func TestResolveAllowlist_CLIOverrides(t *testing.T) {
	t.Parallel()

	userCfg := &UserConfig{EgressAllow: []string{"user.example.com:443"}}
	builtIn := []string{"built-in.example.com:443"}

	result, err := ResolveAllowlist(
		[]string{"cli.example.com:443"},
		userCfg,
		builtIn,
		false,
		"proxy",
	)
	require.NoError(t, err)
	require.Equal(t, "cli", result.Source)
	require.Equal(t, []string{"cli.example.com:443"}, result.Destinations)
}

func TestResolveAllowlist_UserConfigOverridesBuiltIn(t *testing.T) {
	t.Parallel()

	userCfg := &UserConfig{EgressAllow: []string{"user.example.com:443"}}
	builtIn := []string{"built-in.example.com:443"}

	result, err := ResolveAllowlist(nil, userCfg, builtIn, false, "proxy")
	require.NoError(t, err)
	require.Equal(t, "user_config", result.Source)
	require.Equal(t, []string{"user.example.com:443"}, result.Destinations)
}

func TestResolveAllowlist_FallsBackToBuiltIn(t *testing.T) {
	t.Parallel()

	builtIn := []string{"built-in.example.com:443"}

	result, err := ResolveAllowlist(nil, nil, builtIn, false, "proxy")
	require.NoError(t, err)
	require.Equal(t, "built_in", result.Source)
	require.Equal(t, builtIn, result.Destinations)
}

func TestResolveAllowlist_NilUserConfig_FallsToBuiltIn(t *testing.T) {
	t.Parallel()

	builtIn := []string{"built-in.example.com:443"}

	result, err := ResolveAllowlist(nil, nil, builtIn, false, "proxy")
	require.NoError(t, err)
	require.Equal(t, "built_in", result.Source)
}

func TestResolveAllowlist_EmptyUserConfig_FallsToBuiltIn(t *testing.T) {
	t.Parallel()

	userCfg := &UserConfig{EgressAllow: []string{}}
	builtIn := []string{"built-in.example.com:443"}

	result, err := ResolveAllowlist(nil, userCfg, builtIn, false, "proxy")
	require.NoError(t, err)
	require.Equal(t, "built_in", result.Source)
}

func TestResolveAllowlist_NoDefaults_WithCLI(t *testing.T) {
	t.Parallel()

	result, err := ResolveAllowlist(
		[]string{"cli.example.com:443"},
		nil,
		[]string{"built-in.example.com:443"},
		true,
		"proxy",
	)
	require.NoError(t, err)
	require.Equal(t, "cli", result.Source)
	require.Equal(t, []string{"cli.example.com:443"}, result.Destinations)
}

func TestResolveAllowlist_NoDefaults_NoCLI_Proxy_Errors(t *testing.T) {
	t.Parallel()

	_, err := ResolveAllowlist(nil, nil, []string{"built-in.example.com:443"}, true, "proxy")
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy mode requires at least one allowlist destination")
}

func TestResolveAllowlist_NoDefaults_NoCLI_Open_OK(t *testing.T) {
	t.Parallel()

	result, err := ResolveAllowlist(nil, nil, []string{"built-in.example.com:443"}, true, "open")
	require.NoError(t, err)
	require.Empty(t, result.Destinations)
	require.Equal(t, "cli", result.Source)
}

func TestResolveAllowlist_InvalidCLIEntry(t *testing.T) {
	t.Parallel()

	_, err := ResolveAllowlist([]string{":443"}, nil, nil, false, "proxy")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty host")
}

func TestResolveAllowlist_InvalidUserConfigEntry(t *testing.T) {
	t.Parallel()

	userCfg := &UserConfig{EgressAllow: []string{"bad host:443"}}
	_, err := ResolveAllowlist(nil, userCfg, nil, false, "proxy")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid host")
}

func TestResolveAllowlist_CLIParsesAndNormalizes(t *testing.T) {
	t.Parallel()

	result, err := ResolveAllowlist(
		[]string{"example.com"},
		nil,
		nil,
		false,
		"proxy",
	)
	require.NoError(t, err)
	require.Equal(t, []string{"example.com:443"}, result.Destinations)
}

func TestResolveAllowlist_UserConfigParsesAndNormalizes(t *testing.T) {
	t.Parallel()

	userCfg := &UserConfig{EgressAllow: []string{"example.com"}}
	result, err := ResolveAllowlist(nil, userCfg, nil, false, "proxy")
	require.NoError(t, err)
	require.Equal(t, []string{"example.com:443"}, result.Destinations)
}

func TestResolveAllowlist_NoDefaults_NoCLI_None_OK(t *testing.T) {
	t.Parallel()

	result, err := ResolveAllowlist(nil, nil, nil, true, "none")
	require.NoError(t, err)
	require.Empty(t, result.Destinations)
}
