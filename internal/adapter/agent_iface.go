package adapter

// Agent configures CLI arguments and metadata for a coding agent.
// Both claudecode.AgentConfig and opencode.AgentConfig implement this
// interface, enabling the factory to dispatch without per-agent field
// extraction.
type Agent interface {
	// Name returns the agent identifier recorded in agent.json (e.g. "claude-code").
	Name() string

	// BinaryName returns the binary name inside the container image (e.g. "claude").
	BinaryName() string

	// Args returns the CLI arguments for the agent binary.
	Args() []string

	// Image returns the resolved container image reference.
	Image() string

	// Requested returns the agent options requested by the user.
	Requested() map[string]any

	// Supported returns which recorded options the selected agent can honor exactly.
	Supported() map[string]bool

	// EnvVars returns environment variables to inject into the container.
	EnvVars() map[string]string

	// UpdateCommand returns the command to install/update the agent into
	// the given prefix directory, or nil if the agent does not support
	// auto-update. Binaries are installed into prefix/bin/.
	UpdateCommand(prefix string) []string

	// VersionCommand returns the command to check the installed agent version.
	VersionCommand() []string
}
