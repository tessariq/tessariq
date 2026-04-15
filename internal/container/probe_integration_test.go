//go:build integration

package container_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/testutil"
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

func TestProbeImageBinary_InvalidImage_ReturnsImagePullError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := container.ProbeImageBinary(ctx, "ghcr.io/tessariq/does-not-exist:v0.0.0", "sh")
	require.Error(t, err)

	// Must be ImagePullError, NOT BinaryNotFoundError.
	var pullErr *container.ImagePullError
	require.True(t, errors.As(err, &pullErr), "expected ImagePullError, got: %T", err)
	require.Contains(t, pullErr.Image, "does-not-exist")

	var binaryErr *container.BinaryNotFoundError
	require.False(t, errors.As(err, &binaryErr), "must not misclassify pull failure as BinaryNotFoundError")
}

func TestProbeImageBinaries_AllExist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := container.ProbeImageBinaries(ctx, "alpine:latest", "sh", "cat")
	require.NoError(t, err)
}

func TestProbeImageBinaries_ReportsFirstMissingBinary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := container.ProbeImageBinaries(ctx, "alpine:latest", "sh", "stdbuf")
	require.Error(t, err)

	var target *container.BinaryNotFoundError
	require.True(t, errors.As(err, &target))
	require.Equal(t, "stdbuf", target.Binary)
	require.Equal(t, "alpine:latest", target.Image)
}

func TestProbeRuntimeIdentity_ReturnsNumericIdentity(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	imgName := testutil.BuildTestImage(t, "runtime-identity", `FROM alpine:latest
RUN addgroup -g 1234 tessariq && adduser -D -u 1234 -G tessariq -h /home/tessariq tessariq
USER tessariq
`)

	identity, err := container.ProbeRuntimeIdentity(context.Background(), imgName, container.TessariqUser)
	require.NoError(t, err)
	require.Equal(t, container.RuntimeIdentity{UID: 1234, GID: 1234}, identity)
}

func TestProbeRuntimeIdentity_MissingUserReturnsTypedError(t *testing.T) {
	t.Parallel()

	identity, err := container.ProbeRuntimeIdentity(context.Background(), "alpine:latest", container.TessariqUser)
	require.Error(t, err)
	require.Equal(t, container.RuntimeIdentity{}, identity)

	var target *container.RuntimeUserNotFoundError
	require.True(t, errors.As(err, &target))
	require.Equal(t, container.TessariqUser, target.User)
	require.Equal(t, "alpine:latest", target.Image)
}
