//go:build integration

package container_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/container"
)

func TestProbeImageBinary_BinaryExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := container.ProbeImageBinary(ctx, "alpine:latest", "sh")
	require.NoError(t, err)
}

func TestProbeImageBinary_BinaryMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := container.ProbeImageBinary(ctx, "alpine:latest", "nonexistent-binary-xyz")
	require.Error(t, err)

	var target *container.BinaryNotFoundError
	require.True(t, errors.As(err, &target))
	require.Equal(t, "nonexistent-binary-xyz", target.Binary)
	require.Equal(t, "alpine:latest", target.Image)
}

func TestProbeImageBinary_ErrorContainsBinaryAndImage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := container.ProbeImageBinary(ctx, "alpine:latest", "missing-agent")
	require.Error(t, err)
	require.Contains(t, err.Error(), `"missing-agent"`)
	require.Contains(t, err.Error(), "alpine:latest")
	require.Contains(t, err.Error(), "--image")
}
