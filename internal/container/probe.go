package container

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// BinaryNotFoundError indicates that the expected binary was not found in the
// container image. The error message includes guidance to use --image.
type BinaryNotFoundError struct {
	Binary string
	Image  string
}

func (e *BinaryNotFoundError) Error() string {
	return fmt.Sprintf("binary %q not found in runtime image %s; use a compatible runtime image or specify --image to override",
		e.Binary, e.Image)
}

// ImagePullError indicates that Docker could not pull the container image.
// The Output field contains the Docker daemon's error message.
type ImagePullError struct {
	Image  string
	Output string
}

func (e *ImagePullError) Error() string {
	return fmt.Sprintf("cannot pull image %s: %s", e.Image, strings.TrimSpace(e.Output))
}

// RuntimeUserNotFoundError indicates that the runtime image does not define the
// expected non-root user that Tessariq launches inside the container.
type RuntimeUserNotFoundError struct {
	User  string
	Image string
}

func (e *RuntimeUserNotFoundError) Error() string {
	return fmt.Sprintf("runtime image %s does not define user %q; use a compatible runtime image or specify --image to override", e.Image, e.User)
}

// RuntimeIdentity is the resolved numeric identity of the named container user.
type RuntimeIdentity struct {
	UID int
	GID int
}

// ProbeImageBinary checks whether binaryName is available inside image by
// running a short-lived container. It returns a *BinaryNotFoundError when the
// binary cannot be found, an *ImagePullError when Docker cannot pull the image,
// or a generic error for other Docker failures.
func ProbeImageBinary(ctx context.Context, image, binaryName string) error {
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--entrypoint", "",
		image,
		"sh", "-c", fmt.Sprintf("command -v %s", binaryName))

	if out, err := cmd.CombinedOutput(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 125 {
				// Exit code 125 is reserved by Docker for daemon errors
				// (image pull denied, manifest unknown, etc.).
				return &ImagePullError{Image: image, Output: string(out)}
			}
			// Any other non-zero exit means the container ran but the
			// binary was not found. `command -v` exits 1 in bash and 127
			// in busybox when the binary is absent.
			return &BinaryNotFoundError{Binary: binaryName, Image: image}
		}
		return fmt.Errorf("probe binary %q in image %s: %s: %w", binaryName, image, string(out), err)
	}
	return nil
}

// ProbeRuntimeIdentity resolves the numeric uid/gid for user inside image.
func ProbeRuntimeIdentity(ctx context.Context, image, user string) (RuntimeIdentity, error) {
	args := buildProbeRuntimeIdentityArgs(image, user)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 125 {
				return RuntimeIdentity{}, &ImagePullError{Image: image, Output: string(out)}
			}
			trimmed := strings.TrimSpace(string(out))
			if isMissingUserOutput(trimmed) {
				return RuntimeIdentity{}, &RuntimeUserNotFoundError{User: user, Image: image}
			}
		}
		return RuntimeIdentity{}, fmt.Errorf("probe runtime user %q in image %s: %s: %w", user, image, strings.TrimSpace(string(out)), err)
	}
	identity, parseErr := parseRuntimeIdentityOutput(string(out))
	if parseErr != nil {
		return RuntimeIdentity{}, fmt.Errorf("parse runtime identity for user %q in image %s: %w", user, image, parseErr)
	}
	return identity, nil
}

// ProbeImageBinaries checks that every required binary is available inside the
// image. It returns the first probe error encountered.
func ProbeImageBinaries(ctx context.Context, image string, binaryNames ...string) error {
	for _, binaryName := range binaryNames {
		if err := ProbeImageBinary(ctx, image, binaryName); err != nil {
			return err
		}
	}
	return nil
}

func buildProbeRuntimeIdentityArgs(image, user string) []string {
	return []string{
		"run", "--rm",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--entrypoint", "",
		image,
		"sh", "-c", fmt.Sprintf("id -u %s && id -g %s", user, user),
	}
}

func parseRuntimeIdentityOutput(out string) (RuntimeIdentity, error) {
	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) != 2 {
		return RuntimeIdentity{}, fmt.Errorf("expected uid and gid, got %q", strings.TrimSpace(out))
	}
	uid, err := strconv.Atoi(lines[0])
	if err != nil {
		return RuntimeIdentity{}, fmt.Errorf("parse uid %q: %w", lines[0], err)
	}
	gid, err := strconv.Atoi(lines[1])
	if err != nil {
		return RuntimeIdentity{}, fmt.Errorf("parse gid %q: %w", lines[1], err)
	}
	return RuntimeIdentity{UID: uid, GID: gid}, nil
}

func isMissingUserOutput(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "no such user") || strings.Contains(lower, "unknown user")
}
