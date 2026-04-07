package adapter

import (
	"fmt"
	"path/filepath"

	"github.com/tessariq/tessariq/internal/adapter/claudecode"
	"github.com/tessariq/tessariq/internal/adapter/opencode"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
)

// AgentProcess wraps a ProcessRunner with agent and runtime metadata.
type AgentProcess struct {
	Process     runner.ProcessRunner
	AgentInfo   AgentInfo
	RuntimeInfo RuntimeInfo
	BinaryName  string // agent binary name inside the container image
}

// NewAgent returns the Agent for the given config.
func NewAgent(cfg run.Config, taskContent string, envVars map[string]string) (Agent, error) {
	switch cfg.Agent {
	case "claude-code":
		return claudecode.New(cfg, taskContent, envVars), nil
	case "opencode":
		return opencode.New(cfg, taskContent, envVars), nil
	default:
		return nil, fmt.Errorf("unsupported agent: %s", cfg.Agent)
	}
}

// NewProcess creates an AgentProcess for the agent specified in cfg.
// The process runs inside a Docker container assembled from the agent config,
// worktree/evidence paths, and discovered auth/config mounts.
// When proxyEnv is non-nil, the container is attached to the proxy network
// and HTTP_PROXY/HTTPS_PROXY environment variables are injected.
func NewProcess(cfg run.Config, taskContent string, runID, worktreePath, evidencePath string,
	authMounts []authmount.MountSpec, configMounts []authmount.MountSpec,
	agentConfigMount, agentConfigMountStatus string, envVars map[string]string,
	proxyEnv *proxy.ProxyEnv, resolvedEgress string) (*AgentProcess, error) {

	imageSource := "reference"
	if cfg.Image != "" {
		imageSource = "custom"
	}

	a, err := NewAgent(cfg, taskContent, envVars)
	if err != nil {
		return nil, err
	}

	agentEnvVars := a.EnvVars()

	// Merge proxy environment variables when proxy mode is active.
	var networkName string
	if proxyEnv != nil {
		networkName = proxyEnv.NetworkName
		agentEnvVars = mergeEnvVars(agentEnvVars, map[string]string{
			"HTTP_PROXY":  proxyEnv.ProxyAddr,
			"HTTPS_PROXY": proxyEnv.ProxyAddr,
			"http_proxy":  proxyEnv.ProxyAddr,
			"https_proxy": proxyEnv.ProxyAddr,
			"NO_PROXY":    "localhost,127.0.0.1",
			"no_proxy":    "localhost,127.0.0.1",
		})
	}

	// Egress "none" uses Docker's built-in none network (loopback only).
	if resolvedEgress == "none" {
		networkName = "none"
	}

	containerCfg := container.Config{
		Name:         run.ContainerName(runID),
		Image:        a.Image(),
		Command:      append([]string{a.BinaryName()}, a.Args()...),
		WorkDir:      "/work",
		User:         "tessariq",
		Env:          agentEnvVars,
		Mounts:       container.AssembleMounts(worktreePath, evidencePath, authMounts, configMounts),
		Interactive:  cfg.Interactive,
		LineBuffered: !cfg.Interactive,
		NetworkName:  networkName,
		WritableDirs: writableDirsForFileMounts(authMounts, configMounts),
	}

	proc := container.New(containerCfg)

	return &AgentProcess{
		Process:     proc,
		AgentInfo:   NewAgentInfo(a.Name(), a.Requested(), a.Supported()),
		RuntimeInfo: NewRuntimeInfo(a.Image(), imageSource, len(authMounts), agentConfigMount, agentConfigMountStatus),
		BinaryName:  a.BinaryName(),
	}, nil
}

// writableDirsForFileMounts returns the unique parent directories of file-level
// auth mounts. Docker creates intermediate directories as root for file-level
// bind mounts, making them unwritable by the container user. The returned paths
// are used to mkdir -p before the agent starts.
func writableDirsForFileMounts(authMounts, configMounts []authmount.MountSpec) []string {
	configTargets := make(map[string]bool, len(configMounts))
	for _, cm := range configMounts {
		configTargets[cm.ContainerPath] = true
	}

	seen := make(map[string]bool)
	var dirs []string
	for _, m := range authMounts {
		// Walk from the file's parent up to ContainerHome, collecting all
		// intermediate directories. Docker creates these as root-owned for
		// file-level bind mounts; each needs a tmpfs to be writable.
		for dir := filepath.Dir(m.ContainerPath); dir != authmount.ContainerHome && dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			// Skip directories already covered by a config directory mount —
			// a bind-mounted directory provides the structure that tmpfs would,
			// and Docker rejects duplicate mount points.
			if configTargets[dir] {
				continue
			}
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}

// mergeEnvVars combines two env var maps. Values in b override values in a.
func mergeEnvVars(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	merged := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		merged[k] = v
	}
	for k, v := range b {
		merged[k] = v
	}
	return merged
}
