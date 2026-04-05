package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/adapter"
	"github.com/tessariq/tessariq/internal/adapter/opencode"
	"github.com/tessariq/tessariq/internal/authmount"
	"github.com/tessariq/tessariq/internal/container"
	"github.com/tessariq/tessariq/internal/git"
	"github.com/tessariq/tessariq/internal/prereq"
	"github.com/tessariq/tessariq/internal/proxy"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/tmux"
	"github.com/tessariq/tessariq/internal/workspace"
)

type runOutput struct {
	RunID         string
	EvidencePath  string
	WorkspacePath string
	ContainerName string
}

func printRunOutput(w io.Writer, out runOutput, attached bool) {
	fmt.Fprintf(w, "run_id: %s\n", out.RunID)
	fmt.Fprintf(w, "evidence_path: %s\n", out.EvidencePath)
	fmt.Fprintf(w, "workspace_path: %s\n", out.WorkspacePath)
	fmt.Fprintf(w, "container_name: %s\n", out.ContainerName)
	if !attached {
		fmt.Fprintf(w, "attach: tessariq attach %s\n", out.RunID)
	}
	fmt.Fprintf(w, "promote: tessariq promote %s\n", out.RunID)
}

func printNonSuccessOutput(w io.Writer, state runner.State, out runOutput) {
	fmt.Fprintf(w, "run_id: %s\n", out.RunID)
	fmt.Fprintf(w, "state: %s\n", state)
	fmt.Fprintf(w, "evidence_path: %s\n", out.EvidencePath)
}

// printFailureOutput prints the minimum evidence locator fields for a
// post-bootstrap failure so users can inspect logs and artifacts.
func printFailureOutput(w io.Writer, runID, evidencePath string) {
	fmt.Fprintf(w, "run_id: %s\n", runID)
	fmt.Fprintf(w, "evidence_path: %s\n", evidencePath)
}

func newRunCmd() *cobra.Command {
	cfg := run.DefaultConfig()

	cmd := &cobra.Command{
		Use:   "run <task-path>",
		Short: "Run a coding agent against a task file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			cfg.TaskPath = args[0]

			checker := prereq.NewChecker()
			if err := checker.CheckCommand("run"); err != nil {
				return err
			}

			if err := checker.CheckDockerDaemon(cmd.Context()); err != nil {
				return err
			}

			root, err := repoRoot(cmd)
			if err != nil {
				return err
			}

			if err := run.ValidateTaskPath(root, cfg.TaskPath); err != nil {
				return err
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			if cfg.Interactive && cfg.Agent == "opencode" {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"note: --interactive is not natively supported by opencode; option recorded in agent.json as not applied\n")
			}

			if err := git.IsClean(cmd.Context(), root); err != nil {
				return err
			}

			absTaskPath := filepath.Join(root, cfg.TaskPath)
			content, err := os.ReadFile(absTaskPath)
			if err != nil {
				return fmt.Errorf("read task file: %w", err)
			}

			taskTitle := run.ExtractTaskTitle(content, cfg.TaskPath)

			baseSHA, err := git.HeadSHA(cmd.Context(), root)
			if err != nil {
				return err
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}

			resolvedEgress := run.ResolveEgressMode(cfg.ResolveEgress())

			allowlistResult, err := resolveRunAllowlist(cfg, homeDir, resolvedEgress)
			if err != nil {
				return err
			}

			runID, evidenceDir, err := run.BootstrapManifest(root, cfg, taskTitle, baseSHA, allowlistResult.Source, time.Now())
			if err != nil {
				return err
			}

			// Print failure details for any post-bootstrap error so users
			// can locate evidence artifacts for debugging.
			defer func() {
				if retErr != nil {
					var termErr *runner.TerminalStateError
					if errors.As(retErr, &termErr) {
						return
					}
					printFailureOutput(cmd.OutOrStdout(), runID, evidenceDir)
				}
			}()

			if err := run.CopyTaskFile(root, cfg.TaskPath, evidenceDir, content); err != nil {
				return err
			}

			wsPath, err := workspace.Provision(cmd.Context(), homeDir, root, runID, evidenceDir, baseSHA)
			if err != nil {
				return err
			}
			cleanupWorktree := true
			defer func() {
				if cleanupWorktree {
					if cErr := workspace.Cleanup(context.Background(), root, wsPath); cErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: worktree cleanup: %s\n", cErr)
					}
				}
			}()

			// Set up proxy topology in proxy mode.
			var proxyEnv *proxy.ProxyEnv
			if resolvedEgress == "proxy" {
				topo := &proxy.Topology{
					RunID:           runID,
					EvidenceDir:     evidenceDir,
					Destinations:    allowlistResult.Destinations,
					AllowlistSource: allowlistResult.Source,
				}
				proxyEnv, err = topo.Setup(cmd.Context())
				if err != nil {
					return fmt.Errorf("proxy topology setup: %w", err)
				}
				defer func() {
					if tdErr := topo.Teardown(context.Background()); tdErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: proxy topology teardown: %s\n", tdErr)
					}
				}()
			}

			authResult, err := authmount.Discover(cfg.Agent, homeDir, runtime.GOOS, authmount.FileExists)
			if err != nil {
				return err
			}

			agentConfigMount := "disabled"
			agentConfigMountStatus := "disabled"
			var containerEnvVars map[string]string

			var configMounts []authmount.MountSpec
			if cfg.MountAgentConfig {
				agentConfigMount = "enabled"
				configResult, configErr := authmount.DiscoverConfigDirs(cfg.Agent, homeDir, authmount.DirExists, authmount.DirReadable)
				if configErr != nil {
					return fmt.Errorf("discover agent config dirs: %w", configErr)
				}
				agentConfigMountStatus = configResult.Status

				switch configResult.Status {
				case "mounted":
					containerEnvVars = configResult.EnvVars
					configMounts = configResult.Mounts
				case "missing_optional":
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: optional config directory for %s not found; continuing with auth mounts only\n", cfg.Agent)
				case "unreadable_optional":
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: optional config directory for %s is not readable; continuing with auth mounts only\n", cfg.Agent)
				}
			}

			agentProc, err := adapter.NewProcess(cfg, string(content), runID, wsPath, evidenceDir,
				authResult.Mounts, configMounts, agentConfigMount, agentConfigMountStatus, containerEnvVars, proxyEnv, resolvedEgress)
			if err != nil {
				return fmt.Errorf("create agent process: %w", err)
			}

			if err := container.ProbeImageBinary(cmd.Context(), agentProc.RuntimeInfo.Image, agentProc.BinaryName); err != nil {
				return fmt.Errorf("agent %s: %w", agentProc.AgentInfo.Agent, err)
			}

			if err := adapter.WriteAgentInfo(evidenceDir, agentProc.AgentInfo); err != nil {
				return fmt.Errorf("write agent info: %w", err)
			}

			if err := adapter.WriteRuntimeInfo(evidenceDir, agentProc.RuntimeInfo); err != nil {
				return fmt.Errorf("write runtime info: %w", err)
			}

			containerName := run.ContainerName(runID)
			sessionName := run.SessionName(runID)
			appendRunningIndexEntry(cmd.ErrOrStderr(), root, evidenceDir)

			printInteractiveNote(cmd.ErrOrStderr(), cfg.Interactive, cfg.Attach, containerName)

			r := &runner.Runner{
				RunID:         runID,
				EvidenceDir:   evidenceDir,
				RepoRoot:      root,
				Config:        cfg,
				Process:       agentProc.Process,
				Session:       &tmux.Starter{},
				SessionName:   sessionName,
				ContainerName: containerName,
			}

			var runErr error
			if cfg.Attach {
				attachFn := attachSessionFn
				attachName := sessionName
				if cfg.Interactive {
					attachFn = attachContainerFn
					attachName = containerName
				}
				runErr = runWithAttach(cmd.Context(), r, attachName, attachFn)
			} else {
				runErr = r.Run(cmd.Context())
			}

			// Generate diff artifacts when changes exist in the worktree.
			warnDiffArtifacts(cmd.ErrOrStderr(), runner.WriteDiffArtifacts(cmd.Context(), evidenceDir, wsPath, baseSHA))

			// Append index entry after run completes (even on failure).
			appendIndexEntry(cmd.ErrOrStderr(), root, evidenceDir)

			// Print blocked egress destinations if proxy mode was active.
			if proxyEnv != nil {
				printBlockedDestinations(cmd.ErrOrStderr(), evidenceDir)
			}

			var termErr *runner.TerminalStateError
			if errors.As(runErr, &termErr) {
				printNonSuccessOutput(cmd.OutOrStdout(), termErr.State, runOutput{
					RunID:        runID,
					EvidencePath: evidenceDir,
				})
				return runErr
			}

			if runErr != nil {
				return runErr
			}

			cleanupWorktree = false

			printRunOutput(cmd.OutOrStdout(), runOutput{
				RunID:         runID,
				EvidencePath:  evidenceDir,
				WorkspacePath: wsPath,
				ContainerName: containerName,
			}, cfg.Attach)

			return nil
		},
	}

	cmd.Flags().Var((*run.DurationValue)(&cfg.Timeout), "timeout", "maximum run duration")
	cmd.Flags().Var((*run.DurationValue)(&cfg.Grace), "grace", "grace period after timeout before kill")
	cmd.Flags().StringVar(&cfg.Agent, "agent", cfg.Agent, "coding agent (claude-code|opencode)")
	cmd.Flags().StringVar(&cfg.Image, "image", cfg.Image, "container image override")
	cmd.Flags().StringVar(&cfg.Model, "model", cfg.Model, "model identifier for the agent")
	cmd.Flags().BoolVar(&cfg.Interactive, "interactive", cfg.Interactive, "require human approval for agent tool use")
	cmd.Flags().StringVar(&cfg.Egress, "egress", cfg.Egress, "egress mode (none|proxy|open|auto)")
	cmd.Flags().BoolVar(&cfg.UnsafeEgress, "unsafe-egress", cfg.UnsafeEgress, "alias for --egress open")
	cmd.Flags().StringArrayVar(&cfg.EgressAllow, "egress-allow", cfg.EgressAllow, "allowed egress destination (repeatable)")
	cmd.Flags().BoolVar(&cfg.EgressNoDefaults, "egress-no-defaults", cfg.EgressNoDefaults, "ignore default allowlists; only --egress-allow entries apply")
	cmd.Flags().StringArrayVar(&cfg.Pre, "pre", cfg.Pre, "pre-command to run before the agent (repeatable)")
	cmd.Flags().StringArrayVar(&cfg.Verify, "verify", cfg.Verify, "verify command to run after the agent (repeatable)")
	cmd.Flags().BoolVar(&cfg.Attach, "attach", cfg.Attach, "attach to the run session immediately")
	cmd.Flags().BoolVar(&cfg.MountAgentConfig, "mount-agent-config", cfg.MountAgentConfig, "mount the agent's default config directory read-only")

	return cmd
}

// resolveAllowlistDeps holds injectable dependencies for allowlist resolution.
type resolveAllowlistDeps struct {
	xdgConfigHome string
	dirExists     func(string) bool
	readFile      func(string) ([]byte, error)
}

// resolveAllowlistCore loads user config and resolves the egress allowlist
// with full precedence: CLI > user_config > built_in. Dependencies are
// injected via deps for testability.
func resolveAllowlistCore(cfg run.Config, homeDir, resolvedEgress string, deps resolveAllowlistDeps) (*run.AllowlistResult, error) {
	// Load user config only when it can influence the resolved allowlist.
	// Skip when: egress is open/none (allowlist unused), CLI entries are
	// present (highest precedence), or --egress-no-defaults is set
	// (discards config and built-in).
	var userCfg *run.UserConfig
	if resolvedEgress == "proxy" && len(cfg.EgressAllow) == 0 && !cfg.EgressNoDefaults {
		configPath := run.UserConfigPath(deps.xdgConfigHome, homeDir, deps.dirExists)
		var err error
		userCfg, err = run.LoadUserConfig(configPath, deps.readFile)
		if err != nil {
			return nil, err
		}
	}

	// Build the built-in allowlist for the agent.
	// Skip provider resolution when a higher-precedence allowlist source
	// (CLI or user config) already determines egress destinations, because
	// ResolveAllowlist will return before consulting the built-in list.
	var agentEndpoints []adapter.Destination
	switch cfg.Agent {
	case "claude-code":
		agentEndpoints = adapter.ClaudeCodeEndpoints()
	case "opencode":
		if resolvedEgress == "proxy" && len(cfg.EgressAllow) == 0 && !cfg.EgressNoDefaults &&
			(userCfg == nil || len(userCfg.EgressAllow) == 0) {
			authPath := filepath.Join(homeDir, ".local", "share", "opencode", "auth.json")
			configDir := filepath.Join(homeDir, ".config", "opencode")
			provInfo, provErr := opencode.ResolveProviderFromPaths(authPath, configDir, deps.readFile)
			if provErr != nil {
				if errors.Is(provErr, os.ErrNotExist) {
					return nil, &authmount.AuthMissingError{Agent: "opencode"}
				}
				return nil, provErr
			}
			agentEndpoints = adapter.OpenCodeEndpoints(provInfo.Host, provInfo.IsOpenCodeHosted)
		}
	}

	fullBuiltIn := adapter.FullBuiltInAllowlist(agentEndpoints)
	builtInStrings := make([]string, len(fullBuiltIn))
	for i, d := range fullBuiltIn {
		builtInStrings[i] = d.String()
	}

	return run.ResolveAllowlist(cfg.EgressAllow, userCfg, builtInStrings, cfg.EgressNoDefaults, resolvedEgress)
}

// resolveRunAllowlist loads user config and resolves the egress allowlist
// with full precedence: CLI > user_config > built_in.
func resolveRunAllowlist(cfg run.Config, homeDir, resolvedEgress string) (*run.AllowlistResult, error) {
	return resolveAllowlistCore(cfg, homeDir, resolvedEgress, resolveAllowlistDeps{
		xdgConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		dirExists: func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && info.IsDir()
		},
		readFile: os.ReadFile,
	})
}

// warnDiffArtifacts emits a warning when diff artifact generation fails.
// Diff failures are non-fatal to avoid blocking runs over transient git or I/O
// errors.
func warnDiffArtifacts(w io.Writer, err error) {
	if err != nil {
		fmt.Fprintf(w, "warning: diff artifacts skipped: %s\n", err)
	}
}

// appendRunningIndexEntry appends the initial running entry so attach can
// resolve live runs through the shared repository index.
func appendRunningIndexEntry(w io.Writer, repoRoot, evidenceDir string) {
	manifest, err := run.ReadManifest(evidenceDir)
	if err != nil {
		fmt.Fprintf(w, "warning: index entry skipped: %s\n", err)
		return
	}

	entry := run.IndexEntryFromManifest(manifest, string(runner.StateRunning))
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	if err := run.AppendIndex(runsDir, entry); err != nil {
		fmt.Fprintf(w, "warning: index entry skipped: %s\n", err)
	}
}

// appendIndexEntry reads the manifest and status from the evidence directory
// and appends an entry to index.jsonl. The index is supplementary to primary
// evidence artifacts, so failures emit a warning instead of failing the run.
func appendIndexEntry(w io.Writer, repoRoot, evidenceDir string) {
	manifest, err := run.ReadManifest(evidenceDir)
	if err != nil {
		fmt.Fprintf(w, "warning: index entry skipped: %s\n", err)
		return
	}

	status, err := runner.ReadStatus(evidenceDir)
	if err != nil {
		fmt.Fprintf(w, "warning: index entry skipped: %s\n", err)
		return
	}

	entry := run.IndexEntryFromManifest(manifest, string(status.State))
	runsDir := filepath.Join(repoRoot, ".tessariq", "runs")
	if err := run.AppendIndex(runsDir, entry); err != nil {
		fmt.Fprintf(w, "warning: index entry skipped: %s\n", err)
	}
}

// printInteractiveNote emits a stderr hint when the user explicitly requested
// --interactive without --attach, so they know how to reach the container.
func printInteractiveNote(w io.Writer, interactive, attach bool, containerName string) {
	if interactive && !attach {
		fmt.Fprintf(w, "note: interactive mode without --attach; use 'docker attach %s' to provide approval input\n", containerName)
	}
}

// attachSessionFn is injectable for testing.
var attachSessionFn = func(ctx context.Context, name string) error {
	return tmux.AttachSession(ctx, name)
}

// attachContainerFn starts and attaches the terminal to a created container
// using docker start -ai, which atomically connects stdin/stdout from the
// moment the container starts — avoiding the race condition where docker
// attach after docker start fails to forward stdin reliably.
//
// The child process is placed in its own foreground process group via
// SysProcAttr.Foreground so that Docker CLI gets exclusive terminal control
// (matching what the shell does when running docker interactively).
var attachContainerFn = func(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "docker", "start", "-ai", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Foreground: true,
		Ctty:       int(os.Stdin.Fd()),
	}
	err := cmd.Run()
	// Restore our process group as the terminal foreground group so
	// subsequent writes to stdout do not trigger SIGTTOU.
	restoreForeground(os.Stdin.Fd())
	if err != nil {
		return fmt.Errorf("docker start -ai %q: %w", name, err)
	}
	return nil
}

// restoreForeground sets the caller's process group as the foreground group
// of the terminal on fd. This must be called after a child with
// SysProcAttr.Foreground exits, otherwise the parent is left as a background
// group and may receive SIGTTOU on terminal writes.
func restoreForeground(fd uintptr) {
	pgrp := int32(syscall.Getpgrp())
	// TIOCSPGRP = tcsetpgrp: set the foreground process group of the terminal.
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCSPGRP, uintptr(unsafe.Pointer(&pgrp)))
}

// runWithAttach runs the runner in a background goroutine, waits for the
// session to be ready, then attaches the terminal. The function blocks until
// the runner completes regardless of whether the attach succeeds.
//
// For interactive mode, runWithAttach creates an exit-code channel so the
// runner can receive the container's exit code from the attach function
// (docker start -ai) instead of using docker wait.
func runWithAttach(ctx context.Context, r *runner.Runner, sessionName string, attachFn func(context.Context, string) error) error {
	sessionReady := make(chan struct{})
	r.SessionReady = sessionReady

	// For interactive mode, wire the exit channel so the runner reads the
	// container exit code from the attach function (docker start -ai).
	var exitCh chan int
	if r.Config.Interactive {
		exitCh = make(chan int, 1)
		r.InteractiveExitCh = exitCh
	}

	var runErr error
	runDone := make(chan struct{})
	go func() {
		runErr = r.Run(ctx)
		close(runDone)
	}()

	select {
	case <-sessionReady:
		attachErr := attachFn(ctx, sessionName)

		// For interactive mode, forward the exit code to the runner.
		if exitCh != nil {
			exitCh <- exitCodeFromError(attachErr)
		}

		<-runDone
		if runErr != nil {
			return runErr
		}
		// In interactive mode the attach error is a container exit code,
		// already handled by the runner via the exit channel.
		if attachErr != nil && exitCh == nil {
			return fmt.Errorf("attach to run session: %w", attachErr)
		}
		return nil
	case <-runDone:
		return runErr
	}
}

// exitCodeFromError extracts the process exit code from an exec.ExitError.
// Returns 0 for nil error, or 1 if the error is not an ExitError.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

// printBlockedDestinations reads egress events and prints guidance for blocked destinations.
func printBlockedDestinations(w io.Writer, evidenceDir string) {
	events, err := proxy.ReadEventsJSONL(evidenceDir)
	if err != nil || len(events) == 0 {
		return
	}

	fmt.Fprintf(w, "\nBlocked egress destinations:\n")
	seen := make(map[string]bool)
	for _, ev := range events {
		key := fmt.Sprintf("%s:%d", ev.Host, ev.Port)
		if seen[key] {
			continue
		}
		seen[key] = true
		fmt.Fprintf(w, "  - %s (blocked: %s)\n", key, ev.Reason)
	}
	fmt.Fprintf(w, "\nTo allow these destinations, use --egress-allow <host:port>.\n")
	fmt.Fprintf(w, "Or add them to ~/.config/tessariq/config.yaml under egress_allow.\n")
	fmt.Fprintf(w, "Or rerun with --unsafe-egress to bypass proxy enforcement.\n")
}
