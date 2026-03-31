package authmount

import (
	"fmt"
	"os"
	"path/filepath"
)

// ContainerHome is the home directory of the non-root user in the reference
// runtime image (see runtime/reference/Dockerfile).
const ContainerHome = "/home/tessariq"

// MountSpec describes one read-only bind mount for auth state.
type MountSpec struct {
	HostPath      string // absolute host path
	ContainerPath string // deterministic in-container path
	ReadOnly      bool   // always true in v0.1.0
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
				HostPath:      configPath,
				ContainerPath: filepath.Join(ContainerHome, ".claude.json"),
				ReadOnly:      true,
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
