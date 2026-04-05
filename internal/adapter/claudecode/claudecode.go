package claudecode

import (
	"github.com/tessariq/tessariq/internal/run"
)

// DefaultImage is the reference runtime image for the Claude Code agent.
// It is provided for quick onboarding and experimentation only.
// Production users should build and maintain their own runtime images
// and pass them via --image. See docs/runtime-images.md.
const DefaultImage = "ghcr.io/tessariq/claude-code@sha256:5a07f3114731414b71663dd15afc6f27604aeb40b05a29df9e36c793b5331967"

// Name is the agent identifier recorded in agent.json.
const Name = "claude-code"

// BinaryName is the expected binary name for the Claude Code agent.
const BinaryName = "claude"

// AgentConfig holds agent-specific CLI arguments and metadata for Claude Code.
// It is a config builder, not a process runner -- the container package handles execution.
type AgentConfig struct {
	args      []string
	image     string
	requested map[string]any
	applied   map[string]bool
	envVars   map[string]string
}

// New creates a Claude Code agent config from the run configuration.
// envVars are additional environment variables injected into the container.
func New(cfg run.Config, taskContent string, envVars map[string]string) *AgentConfig {
	return &AgentConfig{
		args:      buildArgs(cfg, taskContent),
		image:     resolveImage(cfg),
		requested: buildRequested(cfg),
		applied:   buildApplied(cfg),
		envVars:   envVars,
	}
}

// Args returns the CLI arguments for the agent binary.
func (a *AgentConfig) Args() []string {
	return a.args
}

// Image returns the resolved container image.
func (a *AgentConfig) Image() string {
	return a.image
}

// Requested returns the agent options requested by the user.
func (a *AgentConfig) Requested() map[string]any {
	return a.requested
}

// Applied returns which requested options were applied exactly.
func (a *AgentConfig) Applied() map[string]bool {
	return a.applied
}

// EnvVars returns the environment variables to inject into the container.
func (a *AgentConfig) EnvVars() map[string]string {
	return a.envVars
}

// buildArgs translates run.Config into claude CLI arguments.
// Non-interactive (default): claude --print --dangerously-skip-permissions [--model M] <task>
// Interactive: claude [--model M]
func buildArgs(cfg run.Config, taskContent string) []string {
	var args []string

	if !cfg.Interactive {
		args = append(args, "--print", "--dangerously-skip-permissions")
	}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	if !cfg.Interactive {
		args = append(args, taskContent)
	}

	return args
}

// buildRequested records which agent options were requested by the user.
func buildRequested(cfg run.Config) map[string]any {
	req := map[string]any{
		"interactive": cfg.Interactive,
	}
	if cfg.Model != "" {
		req["model"] = cfg.Model
	}
	return req
}

// buildApplied records which requested options the agent applied exactly.
// Claude Code supports both --model and --interactive natively.
func buildApplied(cfg run.Config) map[string]bool {
	app := map[string]bool{
		"interactive": true,
	}
	if cfg.Model != "" {
		app["model"] = true
	}
	return app
}

// resolveImage returns the container image to use for the run.
func resolveImage(cfg run.Config) string {
	if cfg.Image != "" {
		return cfg.Image
	}
	return DefaultImage
}
