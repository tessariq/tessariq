package adapter

import (
	"fmt"

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
}

// NewProcess creates an AgentProcess for the agent specified in cfg.
// The process runs inside a Docker container assembled from the agent config,
// worktree/evidence paths, and discovered auth/config mounts.
// When proxyEnv is non-nil, the container is attached to the proxy network
// and HTTP_PROXY/HTTPS_PROXY environment variables are injected.
func NewProcess(cfg run.Config, taskContent string, runID, worktreePath, evidencePath string,
	authMounts []authmount.MountSpec, configMounts []authmount.MountSpec,
	agentConfigMount, agentConfigMountStatus string, envVars map[string]string,
	proxyEnv *proxy.ProxyEnv) (*AgentProcess, error) {

	imageSource := "reference"
	if cfg.Image != "" {
		imageSource = "custom"
	}

	var binaryName string
	var agentName string
	var args []string
	var image string
	var requested map[string]any
	var applied map[string]bool
	var agentEnvVars map[string]string

	switch cfg.Agent {
	case "claude-code":
		a := claudecode.New(cfg, taskContent, envVars)
		binaryName = claudecode.BinaryName
		agentName = claudecode.Name
		args = a.Args()
		image = a.Image()
		requested = a.Requested()
		applied = a.Applied()
		agentEnvVars = a.EnvVars()
	case "opencode":
		a := opencode.New(cfg, taskContent, envVars)
		binaryName = opencode.BinaryName
		agentName = opencode.Name
		args = a.Args()
		image = a.Image()
		requested = a.Requested()
		applied = a.Applied()
		agentEnvVars = a.EnvVars()
	default:
		return nil, fmt.Errorf("unsupported agent: %s", cfg.Agent)
	}

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

	containerCfg := container.Config{
		Name:        run.ContainerName(runID),
		Image:       image,
		Command:     append([]string{binaryName}, args...),
		WorkDir:     "/work",
		User:        "tessariq",
		Env:         agentEnvVars,
		Mounts:      container.AssembleMounts(worktreePath, evidencePath, authMounts, configMounts),
		Interactive: cfg.Interactive,
		NetworkName: networkName,
	}

	proc := container.New(containerCfg)

	return &AgentProcess{
		Process:     proc,
		AgentInfo:   NewAgentInfo(agentName, requested, applied),
		RuntimeInfo: NewRuntimeInfo(image, imageSource, len(authMounts), agentConfigMount, agentConfigMountStatus),
	}, nil
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
