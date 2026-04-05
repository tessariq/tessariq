package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

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

func printRunOutput(w io.Writer, out runOutput) {
	fmt.Fprintf(w, "run_id: %s\n", out.RunID)
	fmt.Fprintf(w, "evidence_path: %s\n", out.EvidencePath)
	fmt.Fprintf(w, "workspace_path: %s\n", out.WorkspacePath)
	fmt.Fprintf(w, "container_name: %s\n", out.ContainerName)
	fmt.Fprintf(w, "attach: tessariq attach %s\n", out.RunID)
	fmt.Fprintf(w, "promote: tessariq promote %s\n", out.RunID)
}

func newRunCmd() *cobra.Command {
	cfg := run.DefaultConfig()

	cmd := &cobra.Command{
		Use:   "run <task-path>",
		Short: "Run a coding agent against a task file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			printInteractiveNote(cmd.ErrOrStderr(), cfg.Interactive, cfg.Attach, sessionName)

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
			runErr := r.Run(cmd.Context())

			// Generate diff artifacts when changes exist in the worktree.
			warnDiffArtifacts(cmd.ErrOrStderr(), runner.WriteDiffArtifacts(cmd.Context(), evidenceDir, wsPath, baseSHA))

			// Append index entry after run completes (even on failure).
			appendIndexEntry(cmd.ErrOrStderr(), root, evidenceDir)

			// Print blocked egress destinations if proxy mode was active.
			if proxyEnv != nil {
				printBlockedDestinations(cmd.ErrOrStderr(), evidenceDir)
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
			})

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
// --interactive without --attach, so they know how to reach the session.
func printInteractiveNote(w io.Writer, interactive, attach bool, sessionName string) {
	if interactive && !attach {
		fmt.Fprintf(w, "note: interactive mode without --attach; use 'tmux attach -t %s' to provide approval input\n", sessionName)
	}
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
