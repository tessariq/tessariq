package container

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinaryNotFoundError_ContainsBinaryAndImage(t *testing.T) {
	t.Parallel()

	err := &BinaryNotFoundError{
		Binary: "claude",
		Image:  "ghcr.io/tessariq/claude-code:latest",
	}

	msg := err.Error()
	require.Contains(t, msg, `"claude"`)
	require.Contains(t, msg, "ghcr.io/tessariq/claude-code:latest")
	require.Contains(t, msg, "--image")
}

func TestBinaryNotFoundError_TypeAssertion(t *testing.T) {
	t.Parallel()

	var err error = &BinaryNotFoundError{
		Binary: "opencode",
		Image:  "ghcr.io/tessariq/opencode:latest",
	}

	var target *BinaryNotFoundError
	require.True(t, errors.As(err, &target))
	require.Equal(t, "opencode", target.Binary)
	require.Equal(t, "ghcr.io/tessariq/opencode:latest", target.Image)
}

func TestImagePullError_ContainsImageAndOutput(t *testing.T) {
	t.Parallel()

	err := &ImagePullError{
		Image:  "ghcr.io/tessariq/claude-code:latest",
		Output: "denied",
	}

	msg := err.Error()
	require.Contains(t, msg, "ghcr.io/tessariq/claude-code:latest")
	require.Contains(t, msg, "denied")
}

func TestImagePullError_TypeAssertion(t *testing.T) {
	t.Parallel()

	var err error = &ImagePullError{
		Image:  "ghcr.io/tessariq/opencode:latest",
		Output: "manifest unknown",
	}

	var target *ImagePullError
	require.True(t, errors.As(err, &target))
	require.Equal(t, "ghcr.io/tessariq/opencode:latest", target.Image)
	require.Equal(t, "manifest unknown", target.Output)
}
