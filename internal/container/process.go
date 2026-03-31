package container

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Process implements runner.ProcessRunner by managing a Docker container lifecycle.
// It uses docker create + docker start + docker wait + docker rm.
type Process struct {
	cfg          Config
	docker       string // path to docker binary
	created      bool
	stdout       *os.File  // optional: pipe container stdout here
	stderr       *os.File  // optional: pipe container stderr here
	stdoutWriter io.Writer // optional: io.Writer target (takes precedence over stdout)
	stderrWriter io.Writer // optional: io.Writer target (takes precedence over stderr)
	logsDone     chan struct{}
}

// New creates a container Process from the given configuration.
func New(cfg Config) *Process {
	return &Process{
		cfg:    cfg,
		docker: "docker",
	}
}

// SetOutput configures where container logs are streamed.
func (p *Process) SetOutput(stdout, stderr *os.File) {
	p.stdout = stdout
	p.stderr = stderr
}

// SetOutputWriter configures io.Writer targets for container log streaming.
// When set, these take precedence over SetOutput's *os.File targets.
func (p *Process) SetOutputWriter(stdout, stderr io.Writer) {
	p.stdoutWriter = stdout
	p.stderrWriter = stderr
}

// Start creates the container and starts it, then streams logs in the background.
// Before creating the container, it makes writable bind-mount sources accessible
// to the container user regardless of host UID.
func (p *Process) Start(ctx context.Context) error {
	if err := p.prepareWritableMounts(); err != nil {
		return fmt.Errorf("prepare writable mounts: %w", err)
	}
	if err := p.create(ctx); err != nil {
		return fmt.Errorf("docker create: %w", err)
	}
	if err := p.start(ctx); err != nil {
		_ = p.remove(context.Background())
		return fmt.Errorf("docker start: %w", err)
	}
	p.streamLogs(ctx)
	return nil
}

// Wait blocks until the container exits and returns its exit code.
// The container is removed after wait completes.
func (p *Process) Wait() (int, error) {
	defer func() { _ = p.remove(context.Background()) }()

	cmd := exec.Command(p.docker, "wait", p.cfg.Name)
	out, err := cmd.Output()
	if err != nil {
		return -1, fmt.Errorf("docker wait: %w", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return -1, fmt.Errorf("parse exit code from docker wait: %w", err)
	}
	p.waitForLogs()
	return code, nil
}

// Signal maps OS signals to Docker commands:
//
//	SIGTERM, SIGINT -> docker stop --time=10
//	SIGKILL         -> docker kill
func (p *Process) Signal(sig os.Signal) error {
	cmdArgs := p.signalCommand(sig)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmdArgs[1], err)
	}
	return nil
}

// signalCommand returns the docker command arguments for the given OS signal.
func (p *Process) signalCommand(sig os.Signal) []string {
	switch sig {
	case syscall.SIGTERM, syscall.SIGINT:
		return []string{p.docker, "stop", "--time=10", p.cfg.Name}
	case syscall.SIGKILL:
		return []string{p.docker, "kill", p.cfg.Name}
	default:
		return []string{p.docker, "kill", "--signal=" + sig.String(), p.cfg.Name}
	}
}

// prepareWritableMounts makes writable bind-mount sources accessible to the
// container user regardless of host UID. It runs chmod -R a+rwX on each mount
// where ReadOnly is false. This is safe because these are disposable directories
// (worktrees) created for a single run.
func (p *Process) prepareWritableMounts() error {
	for _, m := range p.cfg.Mounts {
		if m.ReadOnly {
			continue
		}
		cmd := exec.Command("chmod", "-R", "a+rwX", m.Source)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("chmod %s: %s: %w", m.Source, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func (p *Process) create(ctx context.Context) error {
	args := p.buildCreateArgs()
	cmd := exec.CommandContext(ctx, p.docker, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	p.created = true
	return nil
}

func (p *Process) start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, p.docker, "start", p.cfg.Name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (p *Process) remove(ctx context.Context) error {
	if !p.created {
		return nil
	}
	cmd := exec.CommandContext(ctx, p.docker, "rm", "-f", p.cfg.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker rm -f: %w", err)
	}
	return nil
}

func (p *Process) streamLogs(ctx context.Context) {
	var stdout, stderr io.Writer
	if p.stdoutWriter != nil {
		stdout = p.stdoutWriter
	} else if p.stdout != nil {
		stdout = p.stdout
	}
	if p.stderrWriter != nil {
		stderr = p.stderrWriter
	} else if p.stderr != nil {
		stderr = p.stderr
	}
	if stdout == nil && stderr == nil {
		return
	}
	p.logsDone = make(chan struct{})
	cmd := exec.CommandContext(ctx, p.docker, "logs", "--follow", p.cfg.Name)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	go func() {
		defer close(p.logsDone)
		_ = cmd.Run()
	}()
}

func (p *Process) waitForLogs() {
	if p.logsDone == nil {
		return
	}
	<-p.logsDone
}

// buildCreateArgs assembles the full docker create argument list.
func (p *Process) buildCreateArgs() []string {
	args := []string{"create"}

	if p.cfg.Interactive {
		args = append(args, "-i", "-t")
	}

	args = append(args, "--name", p.cfg.Name)

	if p.cfg.User != "" {
		args = append(args, "--user", p.cfg.User)
	}

	if p.cfg.WorkDir != "" {
		args = append(args, "--workdir", p.cfg.WorkDir)
	}

	for _, m := range p.cfg.Mounts {
		args = append(args, "-v", m.DockerFlag())
	}

	for k, v := range p.cfg.Env {
		args = append(args, "--env", k+"="+v)
	}

	args = append(args, p.cfg.Image)
	args = append(args, p.cfg.Command...)

	return args
}
