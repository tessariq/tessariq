package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeCodeEndpoints_ExactlyThree(t *testing.T) {
	t.Parallel()

	endpoints := ClaudeCodeEndpoints()
	require.Len(t, endpoints, 3)
}

func TestClaudeCodeEndpoints_RequiredHosts(t *testing.T) {
	t.Parallel()

	endpoints := ClaudeCodeEndpoints()
	hosts := make(map[string]bool)
	for _, ep := range endpoints {
		hosts[ep.Host] = true
		require.Equal(t, 443, ep.Port)
	}

	require.True(t, hosts["api.anthropic.com"])
	require.True(t, hosts["claude.ai"])
	require.True(t, hosts["platform.claude.com"])
}

func TestAgentEndpoints_ClaudeCode(t *testing.T) {
	t.Parallel()

	endpoints := AgentEndpoints("claude-code")
	require.Len(t, endpoints, 3)
}

func TestAgentEndpoints_OpenCode_ReturnsNil(t *testing.T) {
	t.Parallel()

	endpoints := AgentEndpoints("opencode")
	require.Nil(t, endpoints)
}

func TestAgentEndpoints_UnknownAgent_ReturnsNil(t *testing.T) {
	t.Parallel()

	endpoints := AgentEndpoints("unknown")
	require.Nil(t, endpoints)
}

func TestOpenCodeEndpoints_NonOpenCodeHosted(t *testing.T) {
	t.Parallel()

	endpoints := OpenCodeEndpoints("api.anthropic.com", false)
	require.Len(t, endpoints, 2)

	hosts := make(map[string]bool)
	for _, ep := range endpoints {
		hosts[ep.Host] = true
		require.Equal(t, 443, ep.Port)
	}
	require.True(t, hosts["models.dev"])
	require.True(t, hosts["api.anthropic.com"])
}

func TestOpenCodeEndpoints_OpenCodeHosted(t *testing.T) {
	t.Parallel()

	endpoints := OpenCodeEndpoints("opencode.ai", true)
	require.Len(t, endpoints, 3)

	hosts := make(map[string]bool)
	for _, ep := range endpoints {
		hosts[ep.Host] = true
		require.Equal(t, 443, ep.Port)
	}
	require.True(t, hosts["models.dev"])
	require.True(t, hosts["opencode.ai"])
}

func TestOpenCodeEndpoints_AllPort443(t *testing.T) {
	t.Parallel()

	for _, includeOC := range []bool{true, false} {
		endpoints := OpenCodeEndpoints("test.com", includeOC)
		for _, ep := range endpoints {
			require.Equal(t, 443, ep.Port)
		}
	}
}

func TestDestination_String(t *testing.T) {
	t.Parallel()

	d := Destination{Host: "api.anthropic.com", Port: 443}
	require.Equal(t, "api.anthropic.com:443", d.String())
}

func TestBaselineEndpoints_Count(t *testing.T) {
	t.Parallel()

	endpoints := BaselineEndpoints()
	require.Len(t, endpoints, 10)
}

func TestBaselineEndpoints_AllPort443(t *testing.T) {
	t.Parallel()

	for _, ep := range BaselineEndpoints() {
		require.Equal(t, 443, ep.Port, "host %s should use port 443", ep.Host)
	}
}

func TestBaselineEndpoints_RequiredHosts(t *testing.T) {
	t.Parallel()

	hosts := make(map[string]bool)
	for _, ep := range BaselineEndpoints() {
		hosts[ep.Host] = true
	}

	required := []string{
		"registry.npmjs.org",
		"pypi.org",
		"files.pythonhosted.org",
		"rubygems.org",
		"crates.io",
		"static.crates.io",
		"proxy.golang.org",
		"sum.golang.org",
		"repo1.maven.org",
		"en.wikipedia.org",
	}
	for _, h := range required {
		require.True(t, hosts[h], "missing required host: %s", h)
	}
}

func TestFullBuiltInAllowlist_ClaudeCode(t *testing.T) {
	t.Parallel()

	full := FullBuiltInAllowlist(ClaudeCodeEndpoints())
	require.Len(t, full, 13) // 10 baseline + 3 claude-code
}

func TestFullBuiltInAllowlist_OpenCode_NonHosted(t *testing.T) {
	t.Parallel()

	full := FullBuiltInAllowlist(OpenCodeEndpoints("api.anthropic.com", false))
	require.Len(t, full, 12) // 10 baseline + 2 opencode
}

func TestFullBuiltInAllowlist_OpenCode_Hosted(t *testing.T) {
	t.Parallel()

	full := FullBuiltInAllowlist(OpenCodeEndpoints("opencode.ai", true))
	require.Len(t, full, 13) // 10 baseline + 3 opencode
}

func TestFullBuiltInAllowlist_PreservesOrder(t *testing.T) {
	t.Parallel()

	full := FullBuiltInAllowlist(ClaudeCodeEndpoints())
	// First 10 should be baseline, last 3 should be agent-specific.
	require.Equal(t, "registry.npmjs.org", full[0].Host)
	require.Equal(t, "api.anthropic.com", full[10].Host)
}
