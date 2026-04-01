package containers

import (
	"bytes"
	"context"
	"fmt"
	"os/user"
	"strings"
	"testing"
	"time"

	testcontainers "github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RunEnv is a Testcontainers-backed environment for end-to-end CLI tests.
// It provides an Alpine container with tmux, git, and a fake claude binary
// so the full tessariq run lifecycle can be exercised without host-tool
// dependencies. A bind-mounted directory allows copying the tessariq binary
// in and reading evidence artifacts out.
type RunEnv struct {
	Container testcontainers.Container
	hostDir   string
}

// StartRunEnv creates an Alpine container with tmux, git, bash, and a fake
// claude binary that exits with claudeExitCode. The container bind-mounts
// t.TempDir() at /work for host-side file exchange.
func StartRunEnv(ctx context.Context, t *testing.T, claudeExitCode int) (*RunEnv, error) {
	return StartRunEnvForBinary(ctx, t, "claude", claudeExitCode)
}

// StartRunEnvForBinary creates an Alpine container with tmux, git, bash, and
// a fake binary at /usr/local/bin/<binaryName> that exits with exitCode. The
// container bind-mounts t.TempDir() at /work for host-side file exchange.
func StartRunEnvForBinary(ctx context.Context, t *testing.T, binaryName string, exitCode int) (*RunEnv, error) {
	t.Helper()

	script := fmt.Sprintf("exit %d", exitCode)
	return StartRunEnvWithScript(ctx, t, binaryName, script)
}

// StartRunEnvWithScript creates an Alpine container with tmux, git, bash, and
// a fake binary at /usr/local/bin/<binaryName> whose body is scriptBody. The
// container bind-mounts t.TempDir() at /work for host-side file exchange.
func StartRunEnvWithScript(ctx context.Context, t *testing.T, binaryName string, scriptBody string) (*RunEnv, error) {
	t.Helper()

	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("resolve current user: %w", err)
	}

	hostDir := t.TempDir()

	mounts := testcontainers.Mounts(
		testcontainers.BindMount(hostDir, "/work"),
		testcontainers.BindMount(hostDir, testcontainers.ContainerMountTarget(hostDir)),
		testcontainers.BindMount("/var/run/docker.sock", "/var/run/docker.sock"),
	)

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		Mounts:     mounts,
		WaitingFor: wait.ForExec([]string{"sh", "-c", "true"}).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start run env container: %w", err)
	}

	t.Cleanup(func() {
		// Fix ownership of bind-mounted files so t.TempDir() cleanup succeeds.
		// The container runs as root but the test process runs as the current user.
		// Use a separate context so a slow chown cannot starve terminate.
		chownCtx, chownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer chownCancel()
		chownCmd := fmt.Sprintf("chown -R %s:%s /work", u.Uid, u.Gid)
		_, _, _ = container.Exec(chownCtx, []string{"sh", "-c", chownCmd}, tcexec.Multiplexed())

		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		_ = container.Terminate(termCtx)
	})

	binPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)
	script := fmt.Sprintf("#!/bin/sh\n%s\n", scriptBody)

	// Install runtime dependencies.
	setupCmds := []string{
		"apk add --no-cache tmux git bash docker-cli docker-cli-buildx",
		"git config --global user.email test@test.com",
		"git config --global user.name Test",
		"git config --global init.defaultBranch main",
		fmt.Sprintf("printf '%%s' '%s' > %s && chmod +x %s",
			strings.ReplaceAll(script, "'", "'\\''"), binPath, binPath),
	}

	for _, cmd := range setupCmds {
		code, _, err := container.Exec(ctx, []string{"sh", "-c", cmd}, tcexec.Multiplexed())
		if err != nil {
			return nil, fmt.Errorf("setup %q: %w", cmd, err)
		}
		if code != 0 {
			return nil, fmt.Errorf("setup %q exited with code %d", cmd, code)
		}
	}

	return &RunEnv{Container: container, hostDir: hostDir}, nil
}

// Dir returns the host-side path to the bind-mounted working directory.
func (r *RunEnv) Dir() string {
	return r.hostDir
}

// Exec runs a command inside the run environment container and returns the
// exit code and combined stdout/stderr output.
func (r *RunEnv) Exec(ctx context.Context, cmd []string) (int, string, error) {
	code, reader, err := r.Container.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return -1, "", fmt.Errorf("exec %v: %w", cmd, err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return code, "", fmt.Errorf("read output of %v: %w", cmd, err)
	}

	return code, strings.TrimSpace(buf.String()), nil
}
