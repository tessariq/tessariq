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

// AgentEndpoints returns the built-in egress endpoints for the given agent.
// These are the HTTPS destinations the agent needs to authenticate and operate.
func AgentEndpoints(agent string) []Destination {
	switch agent {
	case "claude-code":
		return ClaudeCodeEndpoints()
	case "opencode":
		return nil
	default:
		return nil
	}
}

// ClaudeCodeEndpoints returns the built-in egress endpoints for Claude Code.
func ClaudeCodeEndpoints() []Destination {
	return []Destination{
		{Host: "api.anthropic.com", Port: 443},
		{Host: "claude.ai", Port: 443},
		{Host: "platform.claude.com", Port: 443},
	}
}
