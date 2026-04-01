//go:build integration

package proxy_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/testutil"
)

// squidDockerfile is used by buildSquidTestImage to build a Squid image
// that includes squidclient (required by the waitForSquid readiness check).
const squidDockerfile = `FROM ubuntu/squid:latest
RUN apt-get update -qq && apt-get install -y -qq squidclient && rm -rf /var/lib/apt/lists/*
`

func buildSquidTestImage(t *testing.T) string {
	t.Helper()
	return testutil.BuildTestImage(t, "squid", squidDockerfile)
}

// curlDockerfile is used by buildCurlTestImage. BusyBox wget does not support
// CONNECT tunneling for HTTPS through a proxy, so we need curl.
const curlDockerfile = `FROM alpine:latest
RUN apk add --no-cache curl
`

func buildCurlTestImage(t *testing.T) string {
	t.Helper()
	return testutil.BuildTestImage(t, "curl", curlDockerfile)
}

// forceCleanup removes the Squid container and Docker network for a run ID,
// ignoring errors (best-effort). This runs even if the test fails.
func forceCleanup(t *testing.T, runID string) {
	t.Helper()
	squidName := proxy.SquidContainerName(runID)
	netName := proxy.NetworkName(runID)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", squidName).Run()
		_ = exec.Command("docker", "network", "rm", netName).Run()
	})
}

func TestIntegration_TopologySetupAndTeardown(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	squidImage := buildSquidTestImage(t)
	runID := testutil.UniqueName(t)
	evidenceDir := t.TempDir()
	forceCleanup(t, runID)

	destinations := []string{"registry.npmjs.org:443", "api.anthropic.com:443"}

	topo := &proxy.Topology{
		RunID:           runID,
		EvidenceDir:     evidenceDir,
		Destinations:    destinations,
		AllowlistSource: "cli",
		SquidImage:      squidImage,
	}

	// --- Setup ---
	env, err := topo.Setup(ctx)
	require.NoError(t, err, "Setup must succeed")

	// Verify ProxyEnv fields.
	require.NotEmpty(t, env.ProxyAddr, "ProxyAddr must be non-empty")
	require.NotEmpty(t, env.NetworkName, "NetworkName must be non-empty")
	require.Contains(t, env.ProxyAddr, "3128", "ProxyAddr should contain the Squid port")

	// Verify egress.compiled.yaml via ReadCompiledYAML.
	compiled, err := proxy.ReadCompiledYAML(evidenceDir)
	require.NoError(t, err, "ReadCompiledYAML must succeed after Setup")
	require.Equal(t, 1, compiled.SchemaVersion, "schema_version must be 1")
	require.Equal(t, "cli", compiled.AllowlistSource, "allowlist_source must match")
	require.Len(t, compiled.Destinations, 2, "compiled destinations count must match input")

	hosts := make([]string, len(compiled.Destinations))
	for i, d := range compiled.Destinations {
		hosts[i] = fmt.Sprintf("%s:%d", d.Host, d.Port)
	}
	require.Contains(t, hosts, "registry.npmjs.org:443")
	require.Contains(t, hosts, "api.anthropic.com:443")

	// --- Teardown ---
	err = topo.Teardown(ctx)
	require.NoError(t, err, "Teardown must succeed")

	// Verify egress.events.jsonl exists (may be empty).
	_, err = proxy.ReadEventsJSONL(evidenceDir)
	require.NoError(t, err, "ReadEventsJSONL must succeed after Teardown")

	// Verify Docker network is removed.
	netName := proxy.NetworkName(runID)
	out, inspectErr := exec.CommandContext(ctx, "docker", "network", "inspect", netName).CombinedOutput()
	require.Error(t, inspectErr, "network should not exist after Teardown: %s", string(out))

	// Verify Squid container is removed.
	squidName := proxy.SquidContainerName(runID)
	out, inspectErr = exec.CommandContext(ctx, "docker", "inspect", squidName).CombinedOutput()
	require.Error(t, inspectErr, "squid container should not exist after Teardown: %s", string(out))
}

func TestIntegration_ProxyAllowsAndBlocks(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	squidImage := buildSquidTestImage(t)
	curlImage := buildCurlTestImage(t)
	runID := testutil.UniqueName(t)
	evidenceDir := t.TempDir()
	forceCleanup(t, runID)

	// Only httpbin.org is allowed; everything else should be blocked.
	topo := &proxy.Topology{
		RunID:           runID,
		EvidenceDir:     evidenceDir,
		Destinations:    []string{"httpbin.org:443"},
		AllowlistSource: "cli",
		SquidImage:      squidImage,
	}

	env, err := topo.Setup(ctx)
	require.NoError(t, err, "Setup must succeed")

	// Create a helper container with curl on the same internal network.
	// BusyBox wget does not use HTTP CONNECT tunneling for HTTPS, so
	// curl is required to properly test HTTPS proxy enforcement.
	alpineName := fmt.Sprintf("tessariq-test-curl-%s", runID)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", alpineName).Run()
	})

	createCmd := exec.CommandContext(ctx, "docker", "create",
		"--name", alpineName,
		"--net", env.NetworkName,
		"--env", "HTTP_PROXY="+env.ProxyAddr,
		"--env", "HTTPS_PROXY="+env.ProxyAddr,
		"--env", "http_proxy="+env.ProxyAddr,
		"--env", "https_proxy="+env.ProxyAddr,
		curlImage,
		"sleep", "300",
	)
	out, err := createCmd.CombinedOutput()
	require.NoError(t, err, "docker create curl container: %s", string(out))

	// Connect to bridge for DNS resolution of external hostnames.
	connectCmd := exec.CommandContext(ctx, "docker", "network", "connect", "bridge", alpineName)
	out, err = connectCmd.CombinedOutput()
	require.NoError(t, err, "docker network connect bridge: %s", string(out))

	startCmd := exec.CommandContext(ctx, "docker", "start", alpineName)
	out, err = startCmd.CombinedOutput()
	require.NoError(t, err, "docker start curl container: %s", string(out))

	// Test 1: Allowed destination (httpbin.org) should succeed.
	// curl uses the CONNECT method for HTTPS through a proxy.
	curlAllowed := exec.CommandContext(ctx, "docker", "exec", alpineName,
		"curl", "-sSf", "-o", "/dev/null",
		"--max-time", "15",
		"--proxy", env.ProxyAddr,
		"https://httpbin.org/get",
	)
	out, err = curlAllowed.CombinedOutput()
	require.NoError(t, err, "curl to httpbin.org (allowed) should succeed: %s", string(out))

	// Test 2: Blocked destination (example.com) should fail.
	curlBlocked := exec.CommandContext(ctx, "docker", "exec", alpineName,
		"curl", "-sSf", "-o", "/dev/null",
		"--max-time", "15",
		"--proxy", env.ProxyAddr,
		"https://example.com/",
	)
	out, err = curlBlocked.CombinedOutput()
	require.Error(t, err, "curl to example.com (blocked) should fail: %s", string(out))

	// Teardown and verify events.
	err = topo.Teardown(ctx)
	require.NoError(t, err, "Teardown must succeed")

	events, err := proxy.ReadEventsJSONL(evidenceDir)
	require.NoError(t, err, "ReadEventsJSONL must succeed")

	// Verify that example.com:443 appears as a blocked event.
	var found bool
	for _, e := range events {
		if e.Host == "example.com" && e.Port == 443 && e.Action == "blocked" {
			found = true
			break
		}
	}
	require.True(t, found, "expected a blocked event for example.com:443, got events: %+v", events)
}
