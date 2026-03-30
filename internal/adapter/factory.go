package adapter

import (
	"fmt"

	"github.com/tessariq/tessariq/internal/adapter/claudecode"
	"github.com/tessariq/tessariq/internal/adapter/opencode"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
)

// AdapterProcess wraps a ProcessRunner with adapter metadata.
type AdapterProcess struct {
	Process  runner.ProcessRunner
	Metadata Info
}

// NewProcess creates an AdapterProcess for the agent specified in cfg.
func NewProcess(cfg run.Config, taskContent string) (*AdapterProcess, error) {
	switch cfg.Agent {
	case "claude-code":
		p := claudecode.New(cfg, taskContent)
		return &AdapterProcess{
			Process: p,
			Metadata: NewInfo(
				claudecode.AdapterName,
				p.Image(),
				p.Requested(),
				p.Applied(),
			),
		}, nil
	case "opencode":
		p := opencode.New(cfg, taskContent)
		return &AdapterProcess{
			Process: p,
			Metadata: NewInfo(
				opencode.AdapterName,
				p.Image(),
				p.Requested(),
				p.Applied(),
			),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported adapter: %s", cfg.Agent)
	}
}
