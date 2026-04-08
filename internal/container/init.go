package container

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// InitConfig holds parameters for the agent auto-update init container.
type InitConfig struct {
	Image         string        // same image as the agent container
	Command       []string      // adapter's UpdateCommand("/cache")
	VersionCmd    []string      // adapter's VersionCommand()
	CacheHostPath string        // host path to ~/.tessariq/agent-cache/<agent>/
	AgentName     string        // for user-facing output
	Timeout       time.Duration // init container timeout (120s default)
}

// InitResult holds the outcome of an init container run.
type InitResult struct {
	Success       bool
	BakedVersion  string
	CachedVersion string
	ElapsedMs     int64
	Error         string
}

// RunInitContainer runs a short-lived init container to update the agent
// binary into the cache directory. On failure, the result records the error
// for evidence and the caller falls back to the baked agent version.
func RunInitContainer(ctx context.Context, cfg InitConfig) InitResult {
	start := time.Now()

	bakedVersion := runVersionProbe(ctx, cfg.Image, cfg.VersionCmd)

	updateCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	args := buildInitRunArgs(cfg)
	cmd := exec.CommandContext(updateCtx, "docker", args...)
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		if updateCtx.Err() == context.DeadlineExceeded {
			errMsg = "init container timed out after " + cfg.Timeout.String()
		}
		// Best-effort: fix ownership of any files the init container created
		// before failing, so the host user can manage the cache directory.
		fixCacheOwnership(ctx, cfg)
		return InitResult{
			BakedVersion: bakedVersion,
			ElapsedMs:    elapsed,
			Error:        errMsg,
		}
	}

	// Fix ownership of cache files created by the root user inside the
	// init container so the host user can read and manage the cache.
	fixCacheOwnership(ctx, cfg)

	cachedVersion := runCacheVersionProbe(ctx, cfg)

	return InitResult{
		Success:       true,
		BakedVersion:  bakedVersion,
		CachedVersion: cachedVersion,
		ElapsedMs:     elapsed,
	}
}

// buildInitRunArgs assembles the docker run arguments for the init container.
// The init container runs as root (for npm global install), with only the cache
// directory mounted. No auth, config, or workdir mounts are provided.
// Capabilities are dropped to the minimum required: DAC_OVERRIDE lets root
// write to the host-owned cache directory, CHOWN lets the ownership fixup
// repair file ownership, and FOWNER lets chmod operate on files regardless
// of ownership context.
func buildInitRunArgs(cfg InitConfig) []string {
	args := []string{"run", "--rm",
		"--cap-drop", "ALL",
		"--cap-add", "DAC_OVERRIDE",
		"--cap-add", "CHOWN",
		"--cap-add", "FOWNER",
		"--security-opt", "no-new-privileges",
		"--user", "root",
		"--entrypoint", "",
		"-v", cfg.CacheHostPath + ":/cache",
		cfg.Image,
	}
	args = append(args, cfg.Command...)
	return args
}

// runVersionProbe runs the version command in a short-lived container and
// returns the trimmed output, or an empty string on failure.
func runVersionProbe(ctx context.Context, image string, versionCmd []string) string {
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{"run", "--rm",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--entrypoint", "",
		image,
	}
	args = append(args, versionCmd...)
	cmd := exec.CommandContext(probeCtx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runCacheVersionProbe runs the version command with the cache mounted and
// PATH layered so the updated binary is found first.
func runCacheVersionProbe(ctx context.Context, cfg InitConfig) string {
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{"run", "--rm",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--entrypoint", "",
		"-v", cfg.CacheHostPath + ":/cache:ro",
		"--env", "PATH=/cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		cfg.Image,
	}
	args = append(args, cfg.VersionCmd...)
	cmd := exec.CommandContext(probeCtx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// fixCacheOwnership runs a short-lived container as root to chmod cache files
// so the host user can manage the cache directory. The init container creates
// files as root, which would be inaccessible to the host user on Linux where
// Docker bind mounts preserve host UIDs.
func fixCacheOwnership(ctx context.Context, cfg InitConfig) {
	fixCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(fixCtx, "docker", "run", "--rm",
		"--cap-drop", "ALL",
		"--cap-add", "DAC_OVERRIDE",
		"--cap-add", "CHOWN",
		"--cap-add", "FOWNER",
		"--security-opt", "no-new-privileges",
		"--entrypoint", "",
		"-v", cfg.CacheHostPath+":/cache",
		cfg.Image,
		"chmod", "-R", "a+rwX", "/cache",
	)
	_ = cmd.Run()
}
