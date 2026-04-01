package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// RequireDocker fails the test immediately if docker is not available on the
// host. Integration tests behind the integration build tag must not silently
// skip when a required dependency is missing — the build tag is the opt-in.
func RequireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatal("docker is required for integration tests but not found in PATH")
	}
}

// RequireTmux fails the test immediately if tmux is not available on the
// host. Integration tests behind the integration build tag must not silently
// skip when a required dependency is missing — the build tag is the opt-in.
func RequireTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatal("tmux is required for integration tests but not found in PATH")
	}
}

// UniqueName returns a Docker-safe, DNS-safe name derived from the test name
// with a nanosecond suffix for collision resistance across parallel runs.
// The name is suitable as a Docker container name or run ID that may be
// further prefixed by domain code (e.g. proxy.SquidContainerName).
func UniqueName(t *testing.T) string {
	t.Helper()
	safe := strings.NewReplacer("/", "-", " ", "-", "_", "-").Replace(t.Name())
	return fmt.Sprintf("%s-%d", strings.ToLower(safe), time.Now().UnixNano()%1_000_000)
}

// BuildTestImage builds a Docker image from an inline Dockerfile string and
// returns the image name. The image is removed in t.Cleanup.
func BuildTestImage(t *testing.T, namePrefix string, dockerfile string) string {
	t.Helper()

	imgName := fmt.Sprintf("tessariq-test-%s-%d", namePrefix, time.Now().UnixNano()%1_000_000)

	buildCmd := exec.Command("docker", "build", "-t", imgName, "-f", "-", ".")
	buildCmd.Stdin = strings.NewReader(dockerfile)
	out, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "build test image %s: %s", imgName, string(out))

	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", imgName).Run()
	})

	return imgName
}

// WithTestTimeout returns a context derived from t.Context() that is cancelled
// after the specified duration or when the test ends, whichever comes first.
func WithTestTimeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), d)
	t.Cleanup(cancel)
	return ctx
}
