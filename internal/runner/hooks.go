package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// HookResult records the outcome of a hook command execution.
type HookResult struct {
	Command  string
	ExitCode int
}

// RunPreHooks executes pre-commands in order. Returns on first failure.
func RunPreHooks(ctx context.Context, commands []string, workDir string, logWriter io.Writer) ([]HookResult, error) {
	var results []HookResult
	for _, cmd := range commands {
		result := runHook(ctx, cmd, workDir, logWriter)
		results = append(results, result)
		if result.ExitCode != 0 {
			return results, fmt.Errorf("pre-command failed: %s (exit %d)", cmd, result.ExitCode)
		}
	}
	return results, nil
}

// RunVerifyHooks executes verify commands in order after agent completion.
// All commands run; failures are collected.
func RunVerifyHooks(ctx context.Context, commands []string, workDir string, logWriter io.Writer) ([]HookResult, error) {
	var results []HookResult
	var firstErr error
	for _, cmd := range commands {
		result := runHook(ctx, cmd, workDir, logWriter)
		results = append(results, result)
		if result.ExitCode != 0 && firstErr == nil {
			firstErr = fmt.Errorf("verify-command failed: %s (exit %d)", cmd, result.ExitCode)
		}
	}
	return results, firstErr
}

func runHook(ctx context.Context, command, workDir string, logWriter io.Writer) HookResult {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return HookResult{Command: command, ExitCode: exitErr.ExitCode()}
		}
		return HookResult{Command: command, ExitCode: -1}
	}

	return HookResult{Command: command, ExitCode: 0}
}
