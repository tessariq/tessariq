package containers

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	testcontainers "github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// AdapterEnv is a Testcontainers-backed environment for adapter integration
// tests. It provides an Alpine container with a configurable fake claude
// binary so tests can exercise adapter process lifecycle in isolation.
type AdapterEnv struct {
	Container testcontainers.Container
}

// StartAdapterEnv creates an Alpine container for adapter integration tests.
// exitCode controls the fake claude binary:
//
//	-1: no binary installed (binary-not-found tests)
//	>= 0: /usr/local/bin/claude exits with that code
func StartAdapterEnv(ctx context.Context, t *testing.T, exitCode int) (*AdapterEnv, error) {
	return StartAdapterEnvForBinary(ctx, t, "claude", exitCode)
}

// StartAdapterEnvForBinary creates an Alpine container with a fake binary
// installed at /usr/local/bin/<binaryName>. exitCode controls behavior:
//
//	-1: no binary installed (binary-not-found tests)
//	>= 0: binary exits with that code
func StartAdapterEnvForBinary(ctx context.Context, t *testing.T, binaryName string, exitCode int) (*AdapterEnv, error) {
	t.Helper()

	if exitCode < 0 {
		return startAdapterContainer(ctx, t, binaryName, "")
	}

	script := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	return startAdapterContainer(ctx, t, binaryName, script)
}

// StartAdapterEnvWithScript creates an Alpine container with a custom claude
// script body. Use this for edge cases like crash-no-output (e.g. "kill -9 $$").
func StartAdapterEnvWithScript(ctx context.Context, t *testing.T, scriptBody string) (*AdapterEnv, error) {
	return StartAdapterEnvWithScriptForBinary(ctx, t, "claude", scriptBody)
}

// StartAdapterEnvWithScriptForBinary creates an Alpine container with a custom
// script body installed as the named binary.
func StartAdapterEnvWithScriptForBinary(ctx context.Context, t *testing.T, binaryName string, scriptBody string) (*AdapterEnv, error) {
	t.Helper()

	script := fmt.Sprintf("#!/bin/sh\n%s\n", scriptBody)
	return startAdapterContainer(ctx, t, binaryName, script)
}

func startAdapterContainer(ctx context.Context, t *testing.T, binaryName string, script string) (*AdapterEnv, error) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"sh", "-c", "true"}).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start adapter container: %w", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	if script != "" {
		binPath := fmt.Sprintf("/usr/local/bin/%s", binaryName)
		installCmd := fmt.Sprintf("printf '%%s' '%s' > %s && chmod +x %s",
			strings.ReplaceAll(script, "'", "'\\''"), binPath, binPath)
		code, _, err := container.Exec(ctx, []string{"sh", "-c", installCmd}, tcexec.Multiplexed())
		if err != nil {
			return nil, fmt.Errorf("install fake %s: %w", binaryName, err)
		}
		if code != 0 {
			return nil, fmt.Errorf("install fake %s exited with code %d", binaryName, code)
		}
	}

	return &AdapterEnv{Container: container}, nil
}

// Exec runs a command inside the adapter container and returns the exit code
// and combined stdout/stderr output.
func (a *AdapterEnv) Exec(ctx context.Context, cmd []string) (int, string, error) {
	code, reader, err := a.Container.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return -1, "", fmt.Errorf("exec %v: %w", cmd, err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return code, "", fmt.Errorf("read output of %v: %w", cmd, err)
	}

	return code, strings.TrimSpace(buf.String()), nil
}
