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
