package main

import (
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
	"github.com/tessariq/tessariq/internal/git"
	"github.com/tessariq/tessariq/internal/prereq"
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

			if err := prereq.NewChecker().CheckCommand("run"); err != nil {
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

			if cfg.Interactive && !cfg.Attach {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: --interactive without --attach; agent will block waiting for approval with no terminal attached")
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

			runID, evidenceDir, err := run.BootstrapManifest(root, cfg, taskTitle, baseSHA, time.Now())
			if err != nil {
				return err
			}

			if err := run.CopyTaskFile(root, cfg.TaskPath, evidenceDir, content); err != nil {
				return err
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}

			wsPath, _, err := workspace.Provision(cmd.Context(), homeDir, root, runID, evidenceDir)
			if err != nil {
				return err
			}

			authResult, err := authmount.Discover(cfg.Agent, homeDir, runtime.GOOS, authmount.FileExists)
			if err != nil {
				return err
			}

			if cfg.Agent == "opencode" && cfg.ResolveEgress() == "auto" {
				authPath := filepath.Join(homeDir, ".local", "share", "opencode", "auth.json")
				configDir := filepath.Join(homeDir, ".config", "opencode")
				if _, provErr := opencode.ResolveProviderFromPaths(authPath, configDir, os.ReadFile); provErr != nil {
					return provErr
				}
			}

			agentConfigMount := "disabled"
			agentConfigMountStatus := "disabled"
			var containerEnvVars map[string]string

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
				case "missing_optional":
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: optional config directory for %s not found; continuing with auth mounts only\n", cfg.Agent)
				case "unreadable_optional":
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: optional config directory for %s is not readable; continuing with auth mounts only\n", cfg.Agent)
				}
			}

			adapterProc, err := adapter.NewProcess(cfg, string(content), len(authResult.Mounts), agentConfigMount, agentConfigMountStatus, containerEnvVars)
			if err != nil {
				return fmt.Errorf("create adapter: %w", err)
			}

			if err := adapter.WriteAgentInfo(evidenceDir, adapterProc.AgentInfo); err != nil {
				return fmt.Errorf("write agent info: %w", err)
			}

			if err := adapter.WriteRuntimeInfo(evidenceDir, adapterProc.RuntimeInfo); err != nil {
				return fmt.Errorf("write runtime info: %w", err)
			}

			containerName := run.ContainerName(runID)
			sessionName := run.SessionName(runID)

			r := &runner.Runner{
				RunID:       runID,
				EvidenceDir: evidenceDir,
				Config:      cfg,
				Process:     adapterProc.Process,
				Session:     &tmux.Starter{},
				SessionName: sessionName,
			}
			if err := r.Run(cmd.Context()); err != nil {
				return err
			}

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
	cmd.Flags().StringVar(&cfg.Agent, "agent", cfg.Agent, "agent adapter (claude-code|opencode)")
	cmd.Flags().StringVar(&cfg.Image, "image", cfg.Image, "container image override")
	cmd.Flags().StringVar(&cfg.Model, "model", cfg.Model, "model identifier for the agent")
	cmd.Flags().BoolVar(&cfg.Interactive, "interactive", cfg.Interactive, "require human approval for agent tool use (use with --attach)")
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
