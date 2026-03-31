package adapter

import "fmt"

// Destination is a host:port pair for egress allowlisting.
type Destination struct {
	Host string
	Port int
}

// String returns the canonical "host:port" representation.
func (d Destination) String() string {
	return fmt.Sprintf("%s:%d", d.Host, d.Port)
}

// AgentEndpoints returns the static built-in egress endpoints for the given agent.
// For agents requiring dynamic provider resolution (opencode), returns nil;
// use OpenCodeEndpoints with resolved provider info instead.
func AgentEndpoints(agent string) []Destination {
	switch agent {
	case "claude-code":
		return ClaudeCodeEndpoints()
	case "opencode":
		return nil // Requires provider resolution; use OpenCodeEndpoints.
	default:
		return nil
	}
}

// OpenCodeEndpoints returns the egress endpoints for OpenCode given resolved
// provider info. Always includes models.dev:443 and the provider host on 443.
// Includes opencode.ai:443 only when includeOpenCodeAI is true.
func OpenCodeEndpoints(providerHost string, includeOpenCodeAI bool) []Destination {
	dests := []Destination{
		{Host: "models.dev", Port: 443},
		{Host: providerHost, Port: 443},
	}
	if includeOpenCodeAI {
		dests = append(dests, Destination{Host: "opencode.ai", Port: 443})
	}
	return dests
}

// BaselineEndpoints returns the static package-manager and reference
// destinations shared across all agents.
func BaselineEndpoints() []Destination {
	return []Destination{
		{Host: "registry.npmjs.org", Port: 443},
		{Host: "pypi.org", Port: 443},
		{Host: "files.pythonhosted.org", Port: 443},
		{Host: "rubygems.org", Port: 443},
		{Host: "crates.io", Port: 443},
		{Host: "static.crates.io", Port: 443},
		{Host: "proxy.golang.org", Port: 443},
		{Host: "sum.golang.org", Port: 443},
		{Host: "repo1.maven.org", Port: 443},
		{Host: "en.wikipedia.org", Port: 443},
	}
}

// FullBuiltInAllowlist returns BaselineEndpoints combined with the provided
// agent-specific endpoints.
func FullBuiltInAllowlist(agentEndpoints []Destination) []Destination {
	baseline := BaselineEndpoints()
	return append(baseline, agentEndpoints...)
}

// ClaudeCodeEndpoints returns the built-in egress endpoints for Claude Code.
func ClaudeCodeEndpoints() []Destination {
	return []Destination{
		{Host: "api.anthropic.com", Port: 443},
		{Host: "claude.ai", Port: 443},
		{Host: "platform.claude.com", Port: 443},
	}
}
