package run

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	require.Equal(t, 30*time.Minute, cfg.Timeout)
	require.Equal(t, 30*time.Second, cfg.Grace)
	require.Equal(t, "claude-code", cfg.Agent)
	require.Equal(t, "auto", cfg.Egress)
	require.False(t, cfg.Attach)
	require.False(t, cfg.UnsafeEgress)
	require.False(t, cfg.Interactive)
	require.False(t, cfg.EgressNoDefaults)
	require.Empty(t, cfg.EgressAllow)
	require.Empty(t, cfg.Pre)
	require.Empty(t, cfg.Verify)
	require.Empty(t, cfg.Image)
	require.Empty(t, cfg.Model)
	require.False(t, cfg.MountAgentConfig)
	require.False(t, cfg.NoUpdateAgent)
}

func TestConfig_Validate_AcceptsDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_MissingTaskPath(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	require.EqualError(t, cfg.Validate(), "task path is required")
}

func TestConfig_Validate_ZeroTimeout(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Timeout = 0
	require.EqualError(t, cfg.Validate(), "timeout must be positive")
}

func TestConfig_Validate_NegativeTimeout(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Timeout = -1
	require.EqualError(t, cfg.Validate(), "timeout must be positive")
}

func TestConfig_Validate_ZeroGrace(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Grace = 0
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_GraceExceedsTimeout(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Timeout = 1 * time.Minute
	cfg.Grace = 2 * time.Minute
	require.EqualError(t, cfg.Validate(), "grace period must not exceed timeout")
}

func TestConfig_Validate_GraceEqualsTimeout(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Timeout = 1 * time.Minute
	cfg.Grace = 1 * time.Minute
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_InvalidAgent(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Agent = "unknown-agent"
	require.EqualError(t, cfg.Validate(), "unsupported agent: unknown-agent")
}

func TestConfig_Validate_ValidAgents(t *testing.T) {
	t.Parallel()

	for _, agent := range []string{"claude-code", "opencode"} {
		cfg := DefaultConfig()
		cfg.TaskPath = "specs/example.md"
		cfg.Agent = agent
		require.NoError(t, cfg.Validate(), "agent: %s", agent)
	}
}

func TestConfig_Validate_InvalidEgress(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Egress = "invalid"
	require.EqualError(t, cfg.Validate(), "unsupported egress mode: invalid")
}

func TestConfig_Validate_ValidEgressModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"none", "proxy", "open", "auto"} {
		cfg := DefaultConfig()
		cfg.TaskPath = "specs/example.md"
		cfg.Egress = mode
		require.NoError(t, cfg.Validate(), "egress: %s", mode)
	}
}

func TestConfig_Validate_EgressNoneWithAllow(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Egress = "none"
	cfg.EgressAllow = []string{"example.com"}
	require.EqualError(t, cfg.Validate(), "egress-allow cannot be used with egress mode none")
}

func TestConfig_Validate_EgressOpenWithAllow(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Egress = "open"
	cfg.EgressAllow = []string{"example.com"}
	require.EqualError(t, cfg.Validate(), "egress-allow cannot be used with egress mode open; allowlists require proxy mode")
}

func TestConfig_Validate_UnsafeEgressWithAllow(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.UnsafeEgress = true
	cfg.EgressAllow = []string{"example.com"}
	require.EqualError(t, cfg.Validate(), "egress-allow cannot be used with egress mode open; allowlists require proxy mode")
}

func TestConfig_Validate_OpenWithoutAllowIsValid(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Egress = "open"
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_AutoWithAllowIsValid(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Egress = "auto"
	cfg.EgressAllow = []string{"example.com"}
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_PreEmptyString(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Pre = []string{""}
	require.EqualError(t, cfg.Validate(), "pre command 0 must not be empty")
}

func TestConfig_Validate_PreWhitespaceOnly(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Pre = []string{"  "}
	require.EqualError(t, cfg.Validate(), "pre command 0 must not be empty")
}

func TestConfig_Validate_VerifyEmptyString(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Verify = []string{""}
	require.EqualError(t, cfg.Validate(), "verify command 0 must not be empty")
}

func TestConfig_Validate_VerifyWhitespaceOnly(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Verify = []string{"  "}
	require.EqualError(t, cfg.Validate(), "verify command 0 must not be empty")
}

func TestConfig_Validate_ValidPreCommands(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Pre = []string{"npm install", "make build"}
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_ValidVerifyCommands(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Verify = []string{"go test ./...", "npm test"}
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_UnsafeEgressWithExplicitEgress(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.TaskPath = "specs/example.md"
	cfg.Egress = "proxy"
	cfg.UnsafeEgress = true
	require.EqualError(t, cfg.Validate(), "unsafe-egress and egress flags are mutually exclusive")
}

func TestResolveEgress_UnsafeEgressOverrides(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.UnsafeEgress = true
	require.Equal(t, "open", cfg.ResolveEgress())
}

func TestResolveEgress_ExplicitEgress(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Egress = "proxy"
	require.Equal(t, "proxy", cfg.ResolveEgress())
}

func TestResolveEgress_DefaultEgress(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	require.Equal(t, "auto", cfg.ResolveEgress())
}
