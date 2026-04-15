package container

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeUserNotFoundError_ContainsUserAndImage(t *testing.T) {
	t.Parallel()

	err := &RuntimeUserNotFoundError{User: TessariqUser, Image: "ghcr.io/tessariq/custom:v1"}
	msg := err.Error()
	require.Contains(t, msg, TessariqUser)
	require.Contains(t, msg, "ghcr.io/tessariq/custom:v1")
	require.Contains(t, msg, "compatible runtime image")
}

func TestRuntimeUserNotFoundError_TypeAssertion(t *testing.T) {
	t.Parallel()

	var err error = &RuntimeUserNotFoundError{User: TessariqUser, Image: "alpine:latest"}
	var target *RuntimeUserNotFoundError
	require.True(t, errors.As(err, &target))
	require.Equal(t, TessariqUser, target.User)
	require.Equal(t, "alpine:latest", target.Image)
}

func TestParseRuntimeIdentityOutput_ReturnsIdentity(t *testing.T) {
	t.Parallel()

	identity, err := parseRuntimeIdentityOutput("1234\n5678\n")
	require.NoError(t, err)
	require.Equal(t, RuntimeIdentity{UID: 1234, GID: 5678}, identity)
}

func TestParseRuntimeIdentityOutput_RejectsMalformedOutput(t *testing.T) {
	t.Parallel()

	_, err := parseRuntimeIdentityOutput("1234\n")
	require.Error(t, err)
	_, err = parseRuntimeIdentityOutput("abc\n5678\n")
	require.Error(t, err)
}

func TestBuildProbeRuntimeIdentityArgs_UsesPinnedSecurityFlags(t *testing.T) {
	t.Parallel()

	args := buildProbeRuntimeIdentityArgs("alpine:latest", TessariqUser)
	require.Equal(t, []string{"run", "--rm", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--entrypoint", "", "alpine:latest", "sh", "-c", "id -u tessariq && id -g tessariq"}, args)
}

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
