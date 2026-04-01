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

// GitRepo is a Testcontainers-backed git repository for integration tests.
// It starts an Alpine container with git installed and bind-mounts a host
// directory so the repo is accessible from both the container and the host.
type GitRepo struct {
	Container testcontainers.Container
	hostDir   string
}

// StartGitRepo creates a Testcontainer with git installed, bind-mounts
// t.TempDir() at /repo, initialises a git repository with one empty commit,
// and returns a handle for running git commands inside the container.
//
// The container runs as the current host user so all files in the bind-mounted
// directory have correct ownership for both container and host-side access.
func StartGitRepo(ctx context.Context, t *testing.T) (*GitRepo, error) {
	t.Helper()

	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("resolve current user: %w", err)
	}

	hostDir := t.TempDir()

	req := testcontainers.ContainerRequest{
		Image:      "alpine/git",
		User:       u.Uid + ":" + u.Gid,
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(hostDir, "/repo"),
		),
		WaitingFor: wait.ForExec([]string{"git", "--version"}).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start git container: %w", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	g := &GitRepo{
		Container: container,
		hostDir:   hostDir,
	}

	if err := g.Exec(ctx, "init", "-b", "main"); err != nil {
		return nil, fmt.Errorf("git init: %w", err)
	}
	if err := g.Exec(ctx, "config", "user.email", "test@example.com"); err != nil {
		return nil, fmt.Errorf("git config email: %w", err)
	}
	if err := g.Exec(ctx, "config", "user.name", "Test User"); err != nil {
		return nil, fmt.Errorf("git config name: %w", err)
	}
	if err := g.Exec(ctx, "commit", "--allow-empty", "-m", "initial"); err != nil {
		return nil, fmt.Errorf("git initial commit: %w", err)
	}

	return g, nil
}

// Dir returns the host-side path to the bind-mounted git repository.
func (g *GitRepo) Dir() string {
	return g.hostDir
}

// Exec runs a git command inside the container's /repo directory.
func (g *GitRepo) Exec(ctx context.Context, args ...string) error {
	cmd := append([]string{"git", "-C", "/repo"}, args...)
	code, _, err := g.Container.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return fmt.Errorf("exec git %v: %w", args, err)
	}
	if code != 0 {
		return fmt.Errorf("git %v exited with code %d", args, code)
	}
	return nil
}

// ExecOutput runs a git command inside the container and returns its
// demuxed stdout as a trimmed string.
func (g *GitRepo) ExecOutput(ctx context.Context, args ...string) (string, error) {
	cmd := append([]string{"git", "-C", "/repo"}, args...)
	code, reader, err := g.Container.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return "", fmt.Errorf("exec git %v: %w", args, err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return "", fmt.Errorf("read git %v output: %w", args, err)
	}

	if code != 0 {
		return "", fmt.Errorf("git %v exited with code %d: %s", args, code, buf.String())
	}

	return strings.TrimSpace(buf.String()), nil
}
