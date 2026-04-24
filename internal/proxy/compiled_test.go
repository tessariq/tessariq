package proxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewCompiledAllowlist_ValidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      string
		dests       []string
		wantLen     int
		wantFirst   CompiledDestination
		wantVersion int
	}{
		{
			name:        "single destination with explicit port",
			source:      "cli",
			dests:       []string{"example.com:443"},
			wantLen:     1,
			wantFirst:   CompiledDestination{Host: "example.com", Port: 443},
			wantVersion: 1,
		},
		{
			name:        "multiple destinations",
			source:      "user_config",
			dests:       []string{"api.example.com:443", "registry.example.com:8443"},
			wantLen:     2,
			wantFirst:   CompiledDestination{Host: "api.example.com", Port: 443},
			wantVersion: 1,
		},
		{
			name:        "default port when omitted",
			source:      "built_in",
			dests:       []string{"example.com"},
			wantLen:     1,
			wantFirst:   CompiledDestination{Host: "example.com", Port: 443},
			wantVersion: 1,
		},
		{
			name:        "bracketed IPv6 with port",
			source:      "cli",
			dests:       []string{"[2001:db8::1]:443"},
			wantLen:     1,
			wantFirst:   CompiledDestination{Host: "2001:db8::1", Port: 443},
			wantVersion: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewCompiledAllowlist(tt.source, tt.dests)
			require.NoError(t, err)
			require.Equal(t, tt.wantVersion, got.SchemaVersion)
			require.Equal(t, tt.source, got.AllowlistSource)
			require.Len(t, got.Destinations, tt.wantLen)
			require.Equal(t, tt.wantFirst, got.Destinations[0])
		})
	}
}

func TestNewCompiledAllowlist_InvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dests     []string
		wantErr   bool
		wantEmpty bool
	}{
		{
			name:      "empty list returns empty destinations",
			dests:     []string{},
			wantErr:   false,
			wantEmpty: true,
		},
		{
			name:    "invalid format returns error",
			dests:   []string{":443"},
			wantErr: true,
		},
		{
			name:    "non-numeric port returns error",
			dests:   []string{"example.com:abc"},
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			dests:   []string{""},
			wantErr: true,
		},
		{
			name:    "bare IPv6 returns error",
			dests:   []string{"2001:db8::1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewCompiledAllowlist("cli", tt.dests)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantEmpty {
				require.Empty(t, got.Destinations)
			}
		})
	}
}

func TestWriteCompiledYAML_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	original := &CompiledAllowlist{
		SchemaVersion:   1,
		AllowlistSource: "cli",
		Destinations: []CompiledDestination{
			{Host: "api.example.com", Port: 443},
			{Host: "registry.example.com", Port: 8443},
		},
	}

	err := WriteCompiledYAML(dir, original)
	require.NoError(t, err)

	got, err := ReadCompiledYAML(dir)
	require.NoError(t, err)

	require.Equal(t, original.SchemaVersion, got.SchemaVersion)
	require.Equal(t, original.AllowlistSource, got.AllowlistSource)
	require.Equal(t, original.Destinations, got.Destinations)
}

func TestWriteCompiledYAML_Permissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := &CompiledAllowlist{
		SchemaVersion:   1,
		AllowlistSource: "cli",
		Destinations: []CompiledDestination{
			{Host: "example.com", Port: 443},
		},
	}

	err := WriteCompiledYAML(dir, c)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "egress.compiled.yaml"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWriteCompiledYAML_Schema(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := &CompiledAllowlist{
		SchemaVersion:   1,
		AllowlistSource: "user_config",
		Destinations: []CompiledDestination{
			{Host: "api.example.com", Port: 443},
			{Host: "registry.example.com", Port: 8443},
		},
	}

	err := WriteCompiledYAML(dir, c)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "egress.compiled.yaml"))
	require.NoError(t, err)

	// Verify YAML structure matches spec format.
	var parsed map[string]any
	err = yaml.Unmarshal(data, &parsed)
	require.NoError(t, err)

	require.Equal(t, 1, parsed["schema_version"])
	require.Equal(t, "user_config", parsed["allowlist_source"])

	dests, ok := parsed["destinations"].([]any)
	require.True(t, ok, "destinations should be a list")
	require.Len(t, dests, 2)

	first, ok := dests[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "api.example.com", first["host"])
	require.Equal(t, 443, first["port"])

	second, ok := dests[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "registry.example.com", second["host"])
	require.Equal(t, 8443, second["port"])
}

func TestCompiledAllowlist_Validate(t *testing.T) {
	t.Parallel()

	valid := &CompiledAllowlist{
		SchemaVersion:   1,
		AllowlistSource: "built_in",
		Destinations:    []CompiledDestination{{Host: "example.com", Port: 443}},
	}
	require.NoError(t, valid.Validate())

	cases := []struct {
		name    string
		c       *CompiledAllowlist
		wantErr string
	}{
		{"bad schema_version", &CompiledAllowlist{SchemaVersion: 0, AllowlistSource: "cli", Destinations: []CompiledDestination{{Host: "x", Port: 443}}}, "schema_version"},
		{"missing allowlist_source", &CompiledAllowlist{SchemaVersion: 1, Destinations: []CompiledDestination{{Host: "x", Port: 443}}}, "allowlist_source"},
		{"empty destinations", &CompiledAllowlist{SchemaVersion: 1, AllowlistSource: "cli", Destinations: []CompiledDestination{}}, "destinations"},
		{"nil destinations", &CompiledAllowlist{SchemaVersion: 1, AllowlistSource: "cli"}, "destinations"},
		{"destination missing host", &CompiledAllowlist{SchemaVersion: 1, AllowlistSource: "cli", Destinations: []CompiledDestination{{Port: 443}}}, "host"},
		{"destination missing port", &CompiledAllowlist{SchemaVersion: 1, AllowlistSource: "cli", Destinations: []CompiledDestination{{Host: "x"}}}, "port"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.c.Validate()
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestReadCompiledYAML_MissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := ReadCompiledYAML(dir)
	require.Error(t, err)
}
