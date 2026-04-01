package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// DefaultSquidImage is the default Docker image for the Squid proxy.
	DefaultSquidImage = "ubuntu/squid:latest"

	// squidListenPort is the port Squid listens on inside the container.
	squidListenPort = 3128
)

// SquidConfig holds everything needed to start the Squid proxy container.
type SquidConfig struct {
	Name        string // container name: tessariq-squid-<run_id>
	Image       string // squid image, defaults to DefaultSquidImage
	NetworkName string // internal network name
	ConfContent string // generated squid.conf content
}

// SquidContainerName returns the deterministic Squid container name for a run.
func SquidContainerName(runID string) string {
	return "tessariq-squid-" + runID
}

// SquidAddress returns the proxy URL for the agent container.
// Since both Squid and the agent are on the same internal network,
// the agent addresses Squid by container name.
func SquidAddress(squidContainerName string) string {
	return fmt.Sprintf("http://%s:%d", squidContainerName, squidListenPort)
}

// StartSquid creates and starts a Squid proxy container.
//
// Steps:
//  1. docker create --name <name> --net <network> <image>
//  2. docker cp squid.conf into the container (avoids bind-mount issues with
//     sibling containers where the host daemon can't see container-local files)
//  3. docker network connect bridge <name> (add outbound internet access)
//  4. docker start <name>
//  5. Wait for readiness (TCP port probe, retry up to 10s with 500ms intervals).
func StartSquid(ctx context.Context, cfg SquidConfig) error {
	// Step 1: docker create (no bind mount for squid.conf).
	image := cfg.Image
	if image == "" {
		image = DefaultSquidImage
	}

	createCmd := exec.CommandContext(ctx, "docker", "create",
		"--name", cfg.Name,
		"--net", cfg.NetworkName,
		image,
	)
	if out, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker create squid: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Step 2: Copy squid.conf into the container via docker cp.
	// Write config to a temp file, then docker cp it in. This avoids the
	// bind-mount path visibility problem in sibling container setups.
	tmpFile, err := os.CreateTemp("", "tessariq-squid-*.conf")
	if err != nil {
		return fmt.Errorf("create squid.conf temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write([]byte(cfg.ConfContent)); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write squid.conf: %w", err)
	}
	tmpFile.Close()

	cpCmd := exec.CommandContext(ctx, "docker", "cp", tmpPath, cfg.Name+":/etc/squid/squid.conf")
	if out, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker cp squid.conf: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Step 3: Connect Squid to the default bridge for outbound internet.
	connectCmd := exec.CommandContext(ctx, "docker", "network", "connect", "bridge", cfg.Name)
	if out, err := connectCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker network connect bridge: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Step 4: docker start.
	startCmd := exec.CommandContext(ctx, "docker", "start", cfg.Name)
	if out, err := startCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker start squid: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Step 5: Wait for readiness.
	if err := waitForSquid(ctx, cfg.Name); err != nil {
		return fmt.Errorf("squid readiness check: %w", err)
	}

	return nil
}

// waitForSquid probes Squid's listen port inside the container up to 10 seconds
// with 500ms intervals. It checks /proc/net/tcp for the listen port, which is
// portable across all Linux images (bash /dev/tcp is not available on
// Debian/Ubuntu where bash is compiled with --disable-net-redirections).
func waitForSquid(ctx context.Context, containerName string) error {
	const (
		timeout  = 10 * time.Second
		interval = 500 * time.Millisecond
	)

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		cmd := exec.CommandContext(ctx, "docker", "exec", containerName,
			"sh", "-c", fmt.Sprintf("grep -q ':%04X' /proc/net/tcp /proc/net/tcp6 2>/dev/null", squidListenPort),
		)
		if _, err := cmd.CombinedOutput(); err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
			}
			continue
		}
		return nil
	}

	return fmt.Errorf("squid not ready after %s: %w", timeout, lastErr)
}

// StopSquid stops and removes the Squid container. Idempotent: if the
// container does not exist, no error is returned.
func StopSquid(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "No such container") ||
			strings.Contains(outStr, "not found") {
			return nil
		}
		return fmt.Errorf("docker rm -f squid: %s: %w", strings.TrimSpace(outStr), err)
	}
	return nil
}

// CopyAccessLog extracts the Squid access log from the container.
// Returns the log content as bytes. If the log file doesn't exist,
// returns empty bytes and nil error.
func CopyAccessLog(ctx context.Context, containerName string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", "exec", containerName,
		"cat", "/var/log/squid/access.log",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// File not found or container gone — return empty.
		return []byte{}, nil
	}
	return out, nil
}
