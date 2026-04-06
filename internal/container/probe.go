package container

import (
	"context"
	"fmt"
	"os/exec"
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
