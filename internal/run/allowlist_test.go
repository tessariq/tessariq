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

func TestParseDestination_ControlCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"newline", "evil\n.example.com:443"},
		{"carriage_return", "evil\r.example.com:443"},
		{"tab", "evil\t.example.com:443"},
		{"nul", "evil\x00.example.com:443"},
		{"del", "evil\x7f.example.com:443"},
		{"soh", "evil\x01.example.com:443"},
		{"form_feed", "evil\x0c.example.com:443"},
		{"vertical_tab", "evil\x0b.example.com:443"},
		{"space", "evil .example.com:443"},
		{"newline_host_only", "evil\n.example.com"},
		{"trailing_newline", "example.com\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseDestination(tt.input)
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid host")
		})
	}
}

func TestParseDestination_ValidHostsStillPass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort int
	}{
		{"simple_domain", "example.com:443", "example.com", 443},
		{"subdomain", "api.example.com:8443", "api.example.com", 8443},
		{"hyphenated", "my-host.example.com:443", "my-host.example.com", 443},
		{"host_only_default_port", "example.com", "example.com", 443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, port, err := ParseDestination(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.wantHost, host)
			require.Equal(t, tt.wantPort, port)
		})
	}
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

func TestParseDestination_IPv6(t *testing.T) {
	t.Parallel()

	t.Run("valid bracketed forms", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			input    string
			wantHost string
			wantPort int
		}{
			{"full_ipv6_with_port", "[2001:db8::1]:443", "2001:db8::1", 443},
			{"loopback_with_port", "[::1]:8443", "::1", 8443},
			{"mapped_ipv4_with_port", "[::ffff:192.0.2.1]:443", "::ffff:192.0.2.1", 443},
			{"bracketed_non_ip_host", "[bad-not-ip]:443", "bad-not-ip", 443},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				host, port, err := ParseDestination(tt.input)
				require.NoError(t, err)
				require.Equal(t, tt.wantHost, host)
				require.Equal(t, tt.wantPort, port)
			})
		}
	})

	t.Run("invalid forms", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			input   string
			wantMsg string
		}{
			{"bare_ipv6_full", "2001:db8::1", "bare IPv6"},
			{"bare_ipv6_loopback", "::1", "bare IPv6"},
			{"bracketed_no_port", "[::1]", "bracketed destination"},
			{"empty_brackets_with_port", "[]:443", "empty host"},
			{"empty_brackets", "[]", "bracketed destination"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, _, err := ParseDestination(tt.input)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantMsg)
			})
		}
	})
}

func TestParseDestination_LeadingDotHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"host_only", ".example.com"},
		{"with_default_port", ".example.com:443"},
		{"with_custom_port", ".github.com:8443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseDestination(tt.input)
			require.Error(t, err)
			require.Contains(t, err.Error(), "leading dot")
		})
	}
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

func TestResolveAllowlist_LeadingDotInCLI(t *testing.T) {
	t.Parallel()

	_, err := ResolveAllowlist([]string{".example.com:443"}, nil, nil, false, "proxy")
	require.Error(t, err)
	require.Contains(t, err.Error(), "leading dot")
}

func TestResolveAllowlist_LeadingDotInUserConfig(t *testing.T) {
	t.Parallel()

	userCfg := &UserConfig{EgressAllow: []string{".example.com:443"}}
	_, err := ResolveAllowlist(nil, userCfg, nil, false, "proxy")
	require.Error(t, err)
	require.Contains(t, err.Error(), "leading dot")
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
