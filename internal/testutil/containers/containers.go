package containers

import (
	"context"
	"fmt"
	"testing"
	"time"

	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartHTTPBin provides a standard Testcontainers-based HTTP dependency for
// integration and end-to-end tests that need a real containerized collaborator.
// The container is automatically terminated via t.Cleanup.
func StartHTTPBin(ctx context.Context, t *testing.T) (testcontainers.Container, string, error) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "kennethreitz/httpbin",
		ExposedPorts: []string{"80/tcp"},
		WaitingFor:   wait.ForHTTP("/get").WithPort("80/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start httpbin container: %w", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolve container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "80/tcp")
	if err != nil {
		return nil, "", fmt.Errorf("resolve container port: %w", err)
	}

	return container, fmt.Sprintf("http://%s:%s", host, port.Port()), nil
}
