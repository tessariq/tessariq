package adapter

import (
	"fmt"

	"github.com/tessariq/tessariq/internal/adapter/claudecode"
	"github.com/tessariq/tessariq/internal/adapter/opencode"
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
// authMountCount is the number of auth mounts discovered by authmount.Discover.
// agentConfigMount and agentConfigMountStatus record the config-dir mount state for runtime.json.
// envVars are additional environment variables injected into the agent process.
func NewProcess(cfg run.Config, taskContent string, authMountCount int, agentConfigMount, agentConfigMountStatus string, envVars map[string]string) (*AgentProcess, error) {
	imageSource := "reference"
	if cfg.Image != "" {
		imageSource = "custom"
	}

	switch cfg.Agent {
	case "claude-code":
		p := claudecode.New(cfg, taskContent, envVars)
		return &AgentProcess{
			Process:     p,
			AgentInfo:   NewAgentInfo(claudecode.Name, p.Requested(), p.Applied()),
			RuntimeInfo: NewRuntimeInfo(p.Image(), imageSource, authMountCount, agentConfigMount, agentConfigMountStatus),
		}, nil
	case "opencode":
		p := opencode.New(cfg, taskContent, envVars)
		return &AgentProcess{
			Process:     p,
			AgentInfo:   NewAgentInfo(opencode.Name, p.Requested(), p.Applied()),
			RuntimeInfo: NewRuntimeInfo(p.Image(), imageSource, authMountCount, agentConfigMount, agentConfigMountStatus),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported agent: %s", cfg.Agent)
	}
}
