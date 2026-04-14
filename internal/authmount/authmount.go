package authmount

import (
	"fmt"
	"os"
	"path/filepath"
)

// ContainerHome is the home directory of the non-root user in the reference
// runtime image (see runtime/reference/Dockerfile).
const ContainerHome = "/home/tessariq"

// AuthMountModeReadOnly is the only valid host-side mount mode for auth and
// config paths under the v0.1.0 contract. Writable agent state is satisfied
// through a disposable per-run runtime-state layer, not through writable host
// binds; see MountSpec.SeedIntoRuntime.
const AuthMountModeReadOnly = "read-only"

// MountSpec describes one host bind mount for auth, config, or state.
//
// Host-side policy is always read-only. When an agent needs writable access
// to a state file (for example Claude Code's startup counter in
// ~/.claude.json), SeedIntoRuntime is set to true; the caller then substitutes
// a disposable per-run scratch file for the host path, so writes never reach
// the live host file.
type MountSpec struct {
	HostPath        string // absolute host path; always bound read-only
	ContainerPath   string // deterministic in-container path the agent expects
	ReadOnly        bool   // host-side policy; always true under the v0.1.0 contract
	SeedIntoRuntime bool   // true = caller must stage a disposable per-run scratch file at ContainerPath and not bind HostPath directly
}

// ValidateContract enforces the v0.1.0 read-only host mount contract: every
// MountSpec MUST have ReadOnly=true. Writability is expressed exclusively
// through SeedIntoRuntime, not through a writable host bind. An adapter that
// returns a writable spec fails fast here rather than silently leaking an
// attack surface for container-to-host persistence via the bound file.
func ValidateContract(specs []MountSpec) error {
	for _, s := range specs {
		if !s.ReadOnly {
			return fmt.Errorf("auth mount contract violated: %s is writable from the container; express writability via SeedIntoRuntime instead", s.HostPath)
		}
	}
	return nil
}

// Result holds the outcome of auth discovery for one agent.
type Result struct {
	Agent  string
	Mounts []MountSpec
}

// Discover resolves required auth files for the given agent on the host.
// goos must be "linux" or "darwin". fileExists checks path existence.
func Discover(agent, homeDir, goos string, fileExists func(string) bool) (*Result, error) {
	switch agent {
	case "claude-code":
		return discoverClaudeCode(homeDir, goos, fileExists)
	case "opencode":
		return discoverOpenCode(homeDir, fileExists)
	default:
		return nil, fmt.Errorf("unsupported agent for auth discovery: %s", agent)
	}
}

func discoverClaudeCode(homeDir, goos string, fileExists func(string) bool) (*Result, error) {
	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	configPath := filepath.Join(homeDir, ".claude.json")

	credExists := fileExists(credPath)
	configExists := fileExists(configPath)

	if !credExists && !configExists {
		return nil, &AuthMissingError{Agent: "claude-code"}
	}

	if !credExists && configExists && goos == "darwin" {
		return nil, &KeychainOnlyError{}
	}

	if !credExists || !configExists {
		return nil, &AuthMissingError{Agent: "claude-code"}
	}

	return &Result{
		Agent: "claude-code",
		Mounts: []MountSpec{
			{
				HostPath:      credPath,
				ContainerPath: filepath.Join(ContainerHome, ".claude", ".credentials.json"),
				ReadOnly:      true,
			},
			{
				// Claude Code mutates .claude.json at startup (numStartups,
				// MCP state, feature flags). The host bind is read-only; a
				// disposable per-run scratch file is seeded with the host
				// contents and bound at ContainerPath so the agent can write
				// freely without any write reaching the live host file.
				HostPath:        configPath,
				ContainerPath:   filepath.Join(ContainerHome, ".claude.json"),
				ReadOnly:        true,
				SeedIntoRuntime: true,
			},
		},
	}, nil
}

func discoverOpenCode(homeDir string, fileExists func(string) bool) (*Result, error) {
	authPath := filepath.Join(homeDir, ".local", "share", "opencode", "auth.json")

	if !fileExists(authPath) {
		return nil, &AuthMissingError{Agent: "opencode"}
	}

	return &Result{
		Agent: "opencode",
		Mounts: []MountSpec{
			{
				HostPath:      authPath,
				ContainerPath: filepath.Join(ContainerHome, ".local", "share", "opencode", "auth.json"),
				ReadOnly:      true,
			},
		},
	}, nil
}

// FileExists is a convenience fileExists implementation using os.Stat.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks whether path exists and is a directory.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// DirReadable checks whether path exists, is a directory, and can be opened.
func DirReadable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// StateResult holds the outcome of optional state-file discovery for one agent.
type StateResult struct {
	Agent  string
	Mounts []MountSpec
	Status string // "mounted", "missing_optional"
}

// DiscoverState resolves optional state files for the given agent.
// State files carry user preferences (e.g. recent model selection) that are not
// required for authentication but affect agent behavior.
func DiscoverState(agent, homeDir string, fileExists func(string) bool) (*StateResult, error) {
	switch agent {
	case "claude-code":
		return &StateResult{Agent: "claude-code", Status: "missing_optional"}, nil
	case "opencode":
		return discoverOpenCodeState(homeDir, fileExists)
	default:
		return nil, fmt.Errorf("unsupported agent for state discovery: %s", agent)
	}
}

func discoverOpenCodeState(homeDir string, fileExists func(string) bool) (*StateResult, error) {
	result := &StateResult{Agent: "opencode"}

	// model.json stores the user's recent model selections. Without it,
	// opencode falls back to the first available provider which may not
	// be the intended one.
	modelPath := filepath.Join(homeDir, ".local", "state", "opencode", "model.json")
	if !fileExists(modelPath) {
		result.Status = "missing_optional"
		return result, nil
	}

	result.Status = "mounted"
	result.Mounts = []MountSpec{
		{
			HostPath:      modelPath,
			ContainerPath: filepath.Join(ContainerHome, ".local", "state", "opencode", "model.json"),
			ReadOnly:      true,
		},
	}
	return result, nil
}

// ConfigDirResult holds the outcome of optional config-dir discovery for one agent.
type ConfigDirResult struct {
	Agent   string
	Mounts  []MountSpec
	Status  string            // "mounted", "missing_optional", "unreadable_optional"
	EnvVars map[string]string // container environment variables to set
}

// DiscoverConfigDirs resolves optional config directories for the given agent.
// dirExists checks path existence as a directory; dirReadable checks readability.
func DiscoverConfigDirs(agent, homeDir string, dirExists, dirReadable func(string) bool) (*ConfigDirResult, error) {
	switch agent {
	case "claude-code":
		return discoverClaudeCodeConfigDirs(homeDir, dirExists, dirReadable)
	case "opencode":
		return discoverOpenCodeConfigDirs(homeDir, dirExists, dirReadable)
	default:
		return nil, fmt.Errorf("unsupported agent for config dir discovery: %s", agent)
	}
}

func discoverClaudeCodeConfigDirs(homeDir string, dirExists, dirReadable func(string) bool) (*ConfigDirResult, error) {
	configDir := filepath.Join(homeDir, ".claude")
	containerDir := filepath.Join(ContainerHome, ".claude")

	result := &ConfigDirResult{Agent: "claude-code"}

	if !dirExists(configDir) {
		result.Status = "missing_optional"
		return result, nil
	}

	if !dirReadable(configDir) {
		result.Status = "unreadable_optional"
		return result, nil
	}

	result.Status = "mounted"
	result.Mounts = []MountSpec{
		{
			HostPath:      configDir,
			ContainerPath: containerDir,
			ReadOnly:      true,
		},
	}
	result.EnvVars = map[string]string{
		"CLAUDE_CONFIG_DIR": containerDir,
	}
	return result, nil
}

func discoverOpenCodeConfigDirs(homeDir string, dirExists, dirReadable func(string) bool) (*ConfigDirResult, error) {
	configDir := filepath.Join(homeDir, ".config", "opencode")
	containerDir := filepath.Join(ContainerHome, ".config", "opencode")

	result := &ConfigDirResult{Agent: "opencode"}

	if !dirExists(configDir) {
		result.Status = "missing_optional"
		return result, nil
	}

	if !dirReadable(configDir) {
		result.Status = "unreadable_optional"
		return result, nil
	}

	result.Status = "mounted"
	result.Mounts = []MountSpec{
		{
			HostPath:      configDir,
			ContainerPath: containerDir,
			ReadOnly:      true,
		},
	}
	return result, nil
}
