package proxy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// NetworkName returns the deterministic Docker network name for a run.
func NetworkName(runID string) string {
	return "tessariq-net-" + runID
}

// CreateNetwork creates an internal Docker bridge network for the run.
// The --internal flag prevents containers on this network from reaching
// the internet directly.
func CreateNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "create", "--internal", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker network create: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoveNetwork removes a Docker network. Idempotent: removing a
// non-existent network is not an error.
func RemoveNetwork(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "network", "rm", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Idempotent: if the network does not exist, treat as success.
		if strings.Contains(string(out), "No such network") ||
			strings.Contains(string(out), "not found") {
			return nil
		}
		return fmt.Errorf("docker network rm: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
