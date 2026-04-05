package opencode

import (
	"fmt"

	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/version"
)

// DefaultImage returns the reference runtime image for the OpenCode agent.
// The image tag matches the CLI version (e.g. ghcr.io/tessariq/opencode:v0.1.0).
// It is provided for quick onboarding and experimentation only.
// Production users should build and maintain their own runtime images
// and pass them via --image. See docs/runtime-images.md.
func DefaultImage() string {
	return fmt.Sprintf("ghcr.io/tessariq/opencode:v%s", version.Version)
}

// Name is the agent identifier recorded in agent.json.
const Name = "opencode"

// BinaryName is the expected binary name for the OpenCode agent.
const BinaryName = "opencode"

// AgentConfig holds agent-specific CLI arguments and metadata for OpenCode.
// It is a config builder, not a process runner -- the container package handles execution.
type AgentConfig struct {
	args      []string
	image     string
	requested map[string]any
	applied   map[string]bool
	envVars   map[string]string
}

// New creates an OpenCode agent config from the run configuration.
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

// buildArgs translates run.Config into opencode CLI arguments.
// Non-interactive (default): opencode run --format json [--model provider/model] -- <task>
// Interactive: opencode -- <task>
func buildArgs(cfg run.Config, taskContent string) []string {
	var args []string

	if !cfg.Interactive {
		args = append(args, "run", "--format", "json")
	}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	args = append(args, "--", taskContent)

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
// OpenCode supports --model natively via the run subcommand.
// Interactive mode falls back to the TUI which does not accept --model.
func buildApplied(cfg run.Config) map[string]bool {
	app := map[string]bool{
		"interactive": false,
	}
	if cfg.Model != "" {
		app["model"] = !cfg.Interactive
	}
	return app
}

// resolveImage returns the container image to use for the run.
func resolveImage(cfg run.Config) string {
	if cfg.Image != "" {
		return cfg.Image
	}
	return DefaultImage()
}
