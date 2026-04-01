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

// SkipIfNoDocker skips the test if docker is not available on the host.
func SkipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
}

// SkipIfNoTmux skips the test if tmux is not available on the host.
func SkipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
}

// UniqueName returns a Docker-safe name derived from the test name with a
// nanosecond suffix for collision resistance across parallel runs.
func UniqueName(t *testing.T) string {
	t.Helper()
	safe := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf("tessariq-test-%s-%d", strings.ToLower(safe), time.Now().UnixNano()%1_000_000)
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
