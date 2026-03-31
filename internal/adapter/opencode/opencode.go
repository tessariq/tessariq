package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/tessariq/tessariq/internal/run"
)

// DefaultImage is the default container image for the OpenCode adapter.
const DefaultImage = "ghcr.io/tessariq/opencode:latest"

// AdapterName is the adapter identifier recorded in adapter.json.
const AdapterName = "opencode"

// BinaryName is the expected binary name for the OpenCode agent.
const BinaryName = "opencode"

// Process implements runner.ProcessRunner for the OpenCode adapter.
type Process struct {
	args      []string
	image     string
	requested map[string]any
	applied   map[string]bool
	envVars   map[string]string
	cmd       *exec.Cmd
}

// New creates an OpenCode adapter process from the run configuration.
// envVars are additional environment variables injected into the process.
func New(cfg run.Config, taskContent string, envVars map[string]string) *Process {
	return &Process{
		args:      buildArgs(cfg, taskContent),
		image:     resolveImage(cfg),
		requested: buildRequested(cfg),
		applied:   buildApplied(cfg),
		envVars:   envVars,
	}
}

// Image returns the resolved container image.
func (p *Process) Image() string {
	return p.image
}

// Requested returns the adapter options requested by the user.
func (p *Process) Requested() map[string]any {
	return p.requested
}

// Applied returns which requested options were applied exactly.
func (p *Process) Applied() map[string]bool {
	return p.applied
}

// Start begins the opencode process.
func (p *Process) Start(ctx context.Context) error {
	p.cmd = exec.CommandContext(ctx, BinaryName, p.args...)
	if len(p.envVars) > 0 {
		p.cmd.Env = os.Environ()
		for k, v := range p.envVars {
			p.cmd.Env = append(p.cmd.Env, k+"="+v)
		}
	}
	err := p.cmd.Start()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("adapter binary %q is not available; ensure the container image includes %s or use --image to specify a compatible image: %w", BinaryName, BinaryName, err)
		}
		return err
	}
	return nil
}

// Wait blocks until the process exits and returns the exit code.
func (p *Process) Wait() (int, error) {
	err := p.cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// Signal sends a signal to the running process.
func (p *Process) Signal(sig os.Signal) error {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Signal(sig)
	}
	return nil
}

// buildArgs translates run.Config into opencode CLI arguments.
// OpenCode takes only the task content as a positional argument.
// It does not support --model or --interactive flags.
func buildArgs(cfg run.Config, taskContent string) []string {
	return []string{taskContent}
}

// buildRequested records which adapter options were requested by the user.
func buildRequested(cfg run.Config) map[string]any {
	req := map[string]any{
		"interactive": cfg.Interactive,
	}
	if cfg.Model != "" {
		req["model"] = cfg.Model
	}
	return req
}

// buildApplied records which requested options the adapter applied exactly.
// OpenCode does not natively support --model or --interactive.
func buildApplied(cfg run.Config) map[string]bool {
	app := map[string]bool{
		"interactive": false,
	}
	if cfg.Model != "" {
		app["model"] = false
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
