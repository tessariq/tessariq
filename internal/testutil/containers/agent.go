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

// AgentEnv is a Testcontainers-backed environment for agent integration
// tests. It provides an Alpine container with a configurable fake agent
// binary so tests can exercise agent process lifecycle in isolation.
type AgentEnv struct {
	Container testcontainers.Container
}

// StartAgentEnv creates an Alpine container for agent integration tests.
// exitCode controls the fake claude binary:
//
//	-1: no binary installed (binary-not-found tests)
//	>= 0: /usr/local/bin/claude exits with that code
func StartAgentEnv(ctx context.Context, t *testing.T, exitCode int) (*AgentEnv, error) {
	return StartAgentEnvForBinary(ctx, t, "claude", exitCode)
}

// StartAgentEnvForBinary creates an Alpine container with a fake binary
// installed at /usr/local/bin/<binaryName>. exitCode controls behavior:
//
//	-1: no binary installed (binary-not-found tests)
//	>= 0: binary exits with that code
func StartAgentEnvForBinary(ctx context.Context, t *testing.T, binaryName string, exitCode int) (*AgentEnv, error) {
	t.Helper()

	if exitCode < 0 {
		return startAgentContainer(ctx, t, binaryName, "")
	}

	script := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	return startAgentContainer(ctx, t, binaryName, script)
}

// StartAgentEnvWithScript creates an Alpine container with a custom claude
// script body. Use this for edge cases like crash-no-output (e.g. "kill -9 $$").
func StartAgentEnvWithScript(ctx context.Context, t *testing.T, scriptBody string) (*AgentEnv, error) {
	return StartAgentEnvWithScriptForBinary(ctx, t, "claude", scriptBody)
}

// StartAgentEnvWithScriptForBinary creates an Alpine container with a custom
// script body installed as the named binary.
func StartAgentEnvWithScriptForBinary(ctx context.Context, t *testing.T, binaryName string, scriptBody string) (*AgentEnv, error) {
	t.Helper()

	script := fmt.Sprintf("#!/bin/sh\n%s\n", scriptBody)
	return startAgentContainer(ctx, t, binaryName, script)
}

func startAgentContainer(ctx context.Context, t *testing.T, binaryName string, script string) (*AgentEnv, error) {
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
		return nil, fmt.Errorf("start agent container: %w", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
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

	return &AgentEnv{Container: container}, nil
}

// Exec runs a command inside the agent container and returns the exit code
// and combined stdout/stderr output.
func (a *AgentEnv) Exec(ctx context.Context, cmd []string) (int, string, error) {
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
