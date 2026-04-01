package container

import (
	"context"
	"fmt"
	"os/exec"
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

// ProbeImageBinary checks whether binaryName is available inside image by
// running a short-lived container. It returns a *BinaryNotFoundError when the
// binary cannot be found, or a generic error for Docker failures.
func ProbeImageBinary(ctx context.Context, image, binaryName string) error {
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--entrypoint", "",
		image,
		"sh", "-c", fmt.Sprintf("command -v %s", binaryName))

	if out, err := cmd.CombinedOutput(); err != nil {
		// Distinguish Docker infrastructure failures from missing binaries.
		// When the probe container runs successfully but the binary is absent,
		// `command -v` exits non-zero — the exit error has a non-nil ExitError.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() > 0 {
			return &BinaryNotFoundError{Binary: binaryName, Image: image}
		}
		return fmt.Errorf("probe binary %q in image %s: %s: %w", binaryName, image, string(out), err)
	}
	return nil
}
