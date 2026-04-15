package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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
	logsCmd      *exec.Cmd
	logsMu       sync.Mutex
}

// New creates a container Process from the given configuration.
func New(cfg Config) *Process {
	return &Process{
		cfg:    cfg,
		docker: "docker",
	}
}

// NetworkName returns the Docker network name configured for this container.
func (p *Process) NetworkName() string { return p.cfg.NetworkName }

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

// StopLogStream stops the background `docker logs --follow` process when it is
// no longer needed, such as after run.log reaches its cap.
func (p *Process) StopLogStream() error {
	p.logsMu.Lock()
	defer p.logsMu.Unlock()
	if p.logsCmd == nil || p.logsCmd.Process == nil {
		return nil
	}
	if err := p.logsCmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop docker logs: %w", err)
	}
	return nil
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
	p.streamLogs()
	return nil
}

// Wait blocks until the container exits and returns its exit code.
func (p *Process) Wait() (int, error) {
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

// Cleanup removes the container after terminal evidence has been persisted.
func (p *Process) Cleanup(ctx context.Context) error {
	return p.remove(ctx)
}

// Signal maps OS signals to Docker commands:
//
//	SIGTERM, SIGINT -> docker kill --signal=<sig> (non-blocking)
//	SIGKILL         -> docker kill (default SIGKILL)
//
// SIGTERM and SIGINT use docker kill --signal rather than docker stop so that
// the caller's grace timer controls escalation instead of Docker's built-in timeout.
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
	case syscall.SIGTERM:
		return []string{p.docker, "kill", "--signal=SIGTERM", p.cfg.Name}
	case syscall.SIGINT:
		return []string{p.docker, "kill", "--signal=SIGINT", p.cfg.Name}
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(trimmed, "No such container") {
			p.created = false
			return nil
		}
		return fmt.Errorf("docker rm -f: %s: %w", trimmed, err)
	}
	p.created = false
	return nil
}

// streamLogs starts a background goroutine that follows container logs until
// the container exits. It intentionally does NOT use the caller's context to
// control the docker-logs process lifetime. docker logs --follow exits
// naturally when the container stops, which ensures that output emitted during
// grace-period shutdown (after timeout) is still captured in run.log.
func (p *Process) streamLogs() {
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
	cmd := exec.Command(p.docker, "logs", "--follow", p.cfg.Name)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	p.logsMu.Lock()
	p.logsCmd = cmd
	p.logsMu.Unlock()
	go func() {
		defer func() {
			p.logsMu.Lock()
			p.logsCmd = nil
			p.logsMu.Unlock()
		}()
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

	args = append(args, "--init")
	args = append(args, "--cap-drop", "ALL")
	args = append(args, "--security-opt", "no-new-privileges")

	if p.cfg.NetworkName != "" {
		args = append(args, "--net", p.cfg.NetworkName)
	}

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

	// WritableDirs become tmpfs mounts so the container user can write
	// to directories that Docker would otherwise create as root for
	// file-level bind mounts. File bind mounts inside these directories
	// nest on top of the tmpfs (Docker resolves mount ordering).
	for _, d := range p.cfg.WritableDirs {
		args = append(args, "--tmpfs", d+":rw,exec")
	}

	args = append(args, p.cfg.Image)

	if p.cfg.LineBuffered {
		args = append(args, "stdbuf", "-oL")
	}
	args = append(args, p.cfg.Command...)

	return args
}
