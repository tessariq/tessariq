//go:build integration

package proxy_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func TestIntegration_SquidContainerSecurityHardening(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	squidImage := buildSquidTestImage(t)
	runID := testutil.UniqueName(t)
	evidenceDir := t.TempDir()
	forceCleanup(t, runID)

	topo := &proxy.Topology{
		RunID:           runID,
		EvidenceDir:     evidenceDir,
		Destinations:    []string{"example.com:443"},
		AllowlistSource: "cli",
		SquidImage:      squidImage,
	}

	_, err := topo.Setup(ctx)
	require.NoError(t, err, "Setup must succeed")

	squidName := proxy.SquidContainerName(runID)

	// Inspect CapDrop.
	capOut, err := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{json .HostConfig.CapDrop}}", squidName).Output()
	require.NoError(t, err, "docker inspect CapDrop")
	require.Contains(t, string(capOut), "ALL",
		"HostConfig.CapDrop must contain ALL")

	// Inspect SecurityOpt.
	secOut, err := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{json .HostConfig.SecurityOpt}}", squidName).Output()
	require.NoError(t, err, "docker inspect SecurityOpt")
	require.Contains(t, string(secOut), "no-new-privileges",
		"HostConfig.SecurityOpt must contain no-new-privileges")

	err = topo.Teardown(ctx)
	require.NoError(t, err, "Teardown must succeed")
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

func TestIntegration_ProxyCrossPortBlocked(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	squidImage := buildSquidTestImage(t)
	curlImage := buildCurlTestImage(t)
	runID := testutil.UniqueName(t)
	evidenceDir := t.TempDir()
	forceCleanup(t, runID)

	// httpbin.org is allowed on 443; example.com is allowed only on 8443.
	// The cross-product (example.com:443) must be blocked.
	topo := &proxy.Topology{
		RunID:           runID,
		EvidenceDir:     evidenceDir,
		Destinations:    []string{"httpbin.org:443", "example.com:8443"},
		AllowlistSource: "cli",
		SquidImage:      squidImage,
	}

	env, err := topo.Setup(ctx)
	require.NoError(t, err, "Setup must succeed")

	alpineName := fmt.Sprintf("tessariq-test-curl-%s", runID)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", alpineName).Run()
	})

	createCmd := exec.CommandContext(ctx, "docker", "create",
		"--name", alpineName,
		"--net", env.NetworkName,
		curlImage,
		"sleep", "300",
	)
	out, err := createCmd.CombinedOutput()
	require.NoError(t, err, "docker create curl container: %s", string(out))

	connectCmd := exec.CommandContext(ctx, "docker", "network", "connect", "bridge", alpineName)
	out, err = connectCmd.CombinedOutput()
	require.NoError(t, err, "docker network connect bridge: %s", string(out))

	startCmd := exec.CommandContext(ctx, "docker", "start", alpineName)
	out, err = startCmd.CombinedOutput()
	require.NoError(t, err, "docker start curl container: %s", string(out))

	// Test 1: Allowed pair (httpbin.org:443) should succeed.
	curlAllowed := exec.CommandContext(ctx, "docker", "exec", alpineName,
		"curl", "-sSf", "-o", "/dev/null",
		"--max-time", "15",
		"--proxy", env.ProxyAddr,
		"https://httpbin.org/get",
	)
	out, err = curlAllowed.CombinedOutput()
	require.NoError(t, err, "curl to httpbin.org:443 (allowed pair) should succeed: %s", string(out))

	// Test 2: Cross-product (example.com:443) must be blocked by Squid.
	// example.com is only allowed on port 8443, not 443.
	curlCross := exec.CommandContext(ctx, "docker", "exec", alpineName,
		"curl", "-sSf", "-o", "/dev/null",
		"--max-time", "15",
		"--proxy", env.ProxyAddr,
		"https://example.com/",
	)
	out, err = curlCross.CombinedOutput()
	require.Error(t, err, "curl to example.com:443 (cross-product) should be blocked: %s", string(out))

	// Test 3: Fully unlisted host must be blocked.
	curlBlocked := exec.CommandContext(ctx, "docker", "exec", alpineName,
		"curl", "-sSf", "-o", "/dev/null",
		"--max-time", "15",
		"--proxy", env.ProxyAddr,
		"https://google.com/",
	)
	out, err = curlBlocked.CombinedOutput()
	require.Error(t, err, "curl to google.com (fully blocked) should fail: %s", string(out))

	// Remove the curl container before teardown so the network has no active endpoints.
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", alpineName).Run()

	err = topo.Teardown(ctx)
	require.NoError(t, err, "Teardown must succeed")
}

func TestIntegration_SetupFailureCleanup(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	// Use a timeout that exceeds the readiness probe timeout (30s).
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	runID := testutil.UniqueName(t)
	forceCleanup(t, runID)

	// Topology.Setup generates config internally, so we can't inject bad config
	// through it. Test StartSquid directly with invalid config instead.

	// Create the network (mimicking what Setup does before StartSquid).
	netName := proxy.NetworkName(runID)
	err := proxy.CreateNetwork(ctx, netName)
	require.NoError(t, err, "CreateNetwork must succeed")

	squidName := proxy.SquidContainerName(runID)
	badCfg := proxy.SquidConfig{
		Name:        squidName,
		NetworkName: netName,
		ConfContent: "invalid_directive_that_makes_squid_exit\n",
	}

	// StartSquid should fail (Squid exits immediately, readiness check fails).
	err = proxy.StartSquid(ctx, badCfg)
	require.Error(t, err, "StartSquid with invalid config must fail")

	// Verify the container was cleaned up.
	out, inspectErr := exec.CommandContext(ctx, "docker", "inspect", squidName).CombinedOutput()
	require.Error(t, inspectErr, "squid container should not exist after failed startup: %s", string(out))

	// Verify the network still exists (StartSquid doesn't clean the network).
	out, inspectErr = exec.CommandContext(ctx, "docker", "network", "inspect", netName).CombinedOutput()
	require.NoError(t, inspectErr, "network should still exist (caller's responsibility): %s", string(out))

	// Clean up the network.
	_ = proxy.RemoveNetwork(ctx, netName)
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

	// Remove the curl container before teardown so the network has no active endpoints.
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", alpineName).Run()

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

func TestIntegration_Teardown_ExtractionFailure_SkipsEvidence_CleansUpInfra(t *testing.T) {
	t.Parallel()
	testutil.RequireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	squidImage := buildSquidTestImage(t)
	runID := testutil.UniqueName(t)
	evidenceDir := t.TempDir()
	forceCleanup(t, runID)

	topo := &proxy.Topology{
		RunID:           runID,
		EvidenceDir:     evidenceDir,
		Destinations:    []string{"example.com:443"},
		AllowlistSource: "cli",
		SquidImage:      squidImage,
	}

	_, err := topo.Setup(ctx)
	require.NoError(t, err, "Setup must succeed")

	// Remove the Squid container before Teardown to force extraction failure.
	squidName := proxy.SquidContainerName(runID)
	rmCmd := exec.CommandContext(ctx, "docker", "rm", "-f", squidName)
	out, err := rmCmd.CombinedOutput()
	require.NoError(t, err, "docker rm -f must succeed: %s", string(out))

	// Teardown should return an error that includes the extraction failure cause.
	tdErr := topo.Teardown(ctx)
	require.Error(t, tdErr, "Teardown must return error when extraction fails")
	require.Contains(t, tdErr.Error(), "telemetry extraction",
		"error must surface telemetry extraction as the root cause")

	// Evidence files must NOT be written (fail-closed).
	_, statErr := os.Stat(filepath.Join(evidenceDir, "egress.events.jsonl"))
	require.True(t, os.IsNotExist(statErr),
		"egress.events.jsonl must not be written when extraction fails")

	// Network cleanup must still run.
	netName := proxy.NetworkName(runID)
	out, inspectErr := exec.CommandContext(ctx, "docker", "network", "inspect", netName).CombinedOutput()
	require.Error(t, inspectErr, "network should be removed after Teardown: %s", string(out))
}
