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

// tlsOriginAlias is the network alias of the in-network TLS origin used as the
// allowlisted destination. Squid resolves it via Docker's embedded DNS on the
// internal network, so the test never depends on external connectivity.
const tlsOriginAlias = "allowed-origin"

// tlsOriginDockerfile builds a minimal nginx origin that serves HTTPS on 443
// with a self-signed certificate. Squid only permits CONNECT tunneling, so the
// allowlisted destination must speak TLS; a plain-HTTP origin cannot validate
// the allow path. The certificate is self-signed, so curl must use --insecure.
const tlsOriginDockerfile = `FROM nginx:alpine
RUN apk add --no-cache openssl \
 && openssl req -x509 -newkey rsa:2048 -nodes -days 825 \
    -keyout /etc/nginx/tls.key -out /etc/nginx/tls.crt -subj "/CN=allowed-origin" \
 && printf 'server {\n  listen 443 ssl;\n  ssl_certificate /etc/nginx/tls.crt;\n  ssl_certificate_key /etc/nginx/tls.key;\n  location / { return 200 "ok\n"; }\n}\n' > /etc/nginx/conf.d/default.conf
`

func buildTLSOriginTestImage(t *testing.T) string {
	t.Helper()
	return testutil.BuildTestImage(t, "tls-origin", tlsOriginDockerfile)
}

// waitForContainerPort blocks until the given TCP port is in LISTEN state inside
// the container, or the context expires. It greps /proc/net/tcp for the port in
// hex, mirroring waitForSquid's portable readiness probe.
func waitForContainerPort(ctx context.Context, t *testing.T, containerName string, port int) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "docker", "exec", containerName,
		"sh", "-c", fmt.Sprintf(
			"while ! grep -q ':%04X' /proc/net/tcp /proc/net/tcp6 2>/dev/null; do sleep 0.5; done",
			port,
		),
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "wait for %s to listen on port %d: %s", containerName, port, string(out))
}

// startTLSOrigin starts the self-signed HTTPS origin on the given internal
// network under tlsOriginAlias and waits for it to listen on 443. It returns
// the container name; cleanup is registered via t.Cleanup. Squid resolves the
// alias via Docker's embedded DNS, so the allow path needs no external network.
func startTLSOrigin(ctx context.Context, t *testing.T, image, networkName, runID string) string {
	t.Helper()
	name := fmt.Sprintf("tessariq-test-origin-%s", runID)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	})

	out, err := exec.CommandContext(ctx, "docker", "create",
		"--name", name,
		"--net", networkName,
		"--network-alias", tlsOriginAlias,
		image,
	).CombinedOutput()
	require.NoError(t, err, "docker create tls origin: %s", string(out))

	out, err = exec.CommandContext(ctx, "docker", "start", name).CombinedOutput()
	require.NoError(t, err, "docker start tls origin: %s", string(out))
	waitForContainerPort(ctx, t, name, 443)
	return name
}

// startCurlContainer starts a long-lived curl helper on the proxy's internal
// network with the proxy environment variables set. It returns the container
// name; cleanup is registered via t.Cleanup. BusyBox wget does not use CONNECT
// tunneling for HTTPS, so curl is required to exercise HTTPS proxy enforcement.
func startCurlContainer(ctx context.Context, t *testing.T, image string, env *proxy.ProxyEnv, runID string) string {
	t.Helper()
	name := fmt.Sprintf("tessariq-test-curl-%s", runID)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	})

	out, err := exec.CommandContext(ctx, "docker", "create",
		"--name", name,
		"--net", env.NetworkName,
		"--env", "HTTP_PROXY="+env.ProxyAddr,
		"--env", "HTTPS_PROXY="+env.ProxyAddr,
		"--env", "http_proxy="+env.ProxyAddr,
		"--env", "https_proxy="+env.ProxyAddr,
		image,
		"sleep", "300",
	).CombinedOutput()
	require.NoError(t, err, "docker create curl container: %s", string(out))

	out, err = exec.CommandContext(ctx, "docker", "start", name).CombinedOutput()
	require.NoError(t, err, "docker start curl container: %s", string(out))
	return name
}

// curlViaProxy runs an HTTPS request from the curl container through the proxy.
// --insecure tolerates the in-network origin's self-signed certificate; these
// tests validate proxy enforcement, not certificate trust.
func curlViaProxy(ctx context.Context, curlContainer, proxyAddr, url string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", "exec", curlContainer,
		"curl", "-sSf", "-k", "-o", "/dev/null",
		"--max-time", "15",
		"--proxy", proxyAddr,
		url,
	).CombinedOutput()
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
	tlsOriginImage := buildTLSOriginTestImage(t)
	runID := testutil.UniqueName(t)
	evidenceDir := t.TempDir()
	forceCleanup(t, runID)

	// The in-network origin (allowed-origin) is allowed on 443; a second host
	// is allowed only on 8443. The cross-product (the second host on 443) must
	// be blocked. Using an in-network origin keeps the allow path deterministic
	// and free of any live external network dependency.
	const crossHost = "cross-host"
	topo := &proxy.Topology{
		RunID:           runID,
		EvidenceDir:     evidenceDir,
		Destinations:    []string{tlsOriginAlias + ":443", crossHost + ":8443"},
		AllowlistSource: "cli",
		SquidImage:      squidImage,
	}

	env, err := topo.Setup(ctx)
	require.NoError(t, err, "Setup must succeed")

	originName := startTLSOrigin(ctx, t, tlsOriginImage, env.NetworkName, runID)
	curlName := startCurlContainer(ctx, t, curlImage, env, runID)

	// Test 1: Allowed pair (allowed-origin:443) should succeed.
	out, err := curlViaProxy(ctx, curlName, env.ProxyAddr, "https://"+tlsOriginAlias+"/")
	require.NoError(t, err, "curl to %s:443 (allowed pair) should succeed: %s", tlsOriginAlias, string(out))

	// Test 2: Cross-product (crossHost:443) must be blocked by Squid — crossHost
	// is only allowed on 8443. Squid denies at the ACL, so no host need exist.
	out, err = curlViaProxy(ctx, curlName, env.ProxyAddr, "https://"+crossHost+"/")
	require.Error(t, err, "curl to %s:443 (cross-product) should be blocked: %s", crossHost, string(out))

	// Test 3: Fully unlisted host must be blocked.
	out, err = curlViaProxy(ctx, curlName, env.ProxyAddr, "https://unlisted-host/")
	require.Error(t, err, "curl to unlisted-host (fully blocked) should fail: %s", string(out))

	// Remove helper containers before teardown so the network has no active endpoints.
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", curlName).Run()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", originName).Run()

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
	tlsOriginImage := buildTLSOriginTestImage(t)
	runID := testutil.UniqueName(t)
	evidenceDir := t.TempDir()
	forceCleanup(t, runID)

	// Only the in-network TLS origin is allowed; everything else is blocked.
	// Using a containerized origin (resolved by Squid via Docker's embedded
	// DNS on the internal network) keeps the test fully deterministic and free
	// of any live external network dependency.
	topo := &proxy.Topology{
		RunID:           runID,
		EvidenceDir:     evidenceDir,
		Destinations:    []string{tlsOriginAlias + ":443"},
		AllowlistSource: "cli",
		SquidImage:      squidImage,
	}

	env, err := topo.Setup(ctx)
	require.NoError(t, err, "Setup must succeed")

	// Start the allowlisted TLS origin and the curl helper on the internal
	// network. Squid resolves the origin's alias via Docker's embedded DNS, so
	// the allow path is exercised without any live external connectivity.
	originName := startTLSOrigin(ctx, t, tlsOriginImage, env.NetworkName, runID)
	curlName := startCurlContainer(ctx, t, curlImage, env, runID)

	// Test 1: Allowed destination (the in-network TLS origin) should succeed.
	// curl uses the CONNECT method for HTTPS through a proxy.
	out, err := curlViaProxy(ctx, curlName, env.ProxyAddr, "https://"+tlsOriginAlias+"/")
	require.NoError(t, err, "curl to %s (allowed) should succeed: %s", tlsOriginAlias, string(out))

	// Test 2: Blocked destination should fail. The host is not allowlisted, so
	// Squid denies the CONNECT at the ACL before any outbound connection — no
	// external network is involved.
	const blockedHost = "blocked-origin"
	out, err = curlViaProxy(ctx, curlName, env.ProxyAddr, "https://"+blockedHost+"/")
	require.Error(t, err, "curl to %s (blocked) should fail: %s", blockedHost, string(out))

	// Remove helper containers before teardown so the network has no active endpoints.
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", curlName).Run()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", originName).Run()

	// Teardown and verify events.
	err = topo.Teardown(ctx)
	require.NoError(t, err, "Teardown must succeed")

	events, err := proxy.ReadEventsJSONL(evidenceDir)
	require.NoError(t, err, "ReadEventsJSONL must succeed")

	// Verify that the blocked host appears as a blocked event.
	var found bool
	for _, e := range events {
		if e.Host == blockedHost && e.Port == 443 && e.Action == "blocked" {
			found = true
			break
		}
	}
	require.True(t, found, "expected a blocked event for %s:443, got events: %+v", blockedHost, events)
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
