package containers

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	testcontainers "github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RuntimeEnv is a Testcontainers-backed environment for reference runtime
// image integration tests. It builds the image from runtime/reference/Dockerfile
// and provides an Exec method for running commands inside the container.
type RuntimeEnv struct {
	Container testcontainers.Container
}

// StartReferenceRuntime builds the reference runtime image from the repo's
// runtime/reference/Dockerfile and starts a container for inspection. The
// image build is cached by Docker layers on subsequent runs.
func StartReferenceRuntime(ctx context.Context, t *testing.T) (*RuntimeEnv, error) {
	t.Helper()

	// Resolve the Dockerfile build context relative to this source file
	// so tests work regardless of the working directory.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	buildCtx := filepath.Join(repoRoot, "runtime", "reference")

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    buildCtx,
			Dockerfile: "Dockerfile",
		},
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"bash", "-c", "true"}).
			WithStartupTimeout(5 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start reference runtime container: %w", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	return &RuntimeEnv{Container: container}, nil
}

// Exec runs a command inside the reference runtime container and returns the
// exit code and combined stdout/stderr output.
func (r *RuntimeEnv) Exec(ctx context.Context, cmd []string) (int, string, error) {
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
