package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/git"
	"github.com/tessariq/tessariq/internal/run"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/workspace"
)

func newRunCmd() *cobra.Command {
	cfg := run.DefaultConfig()

	cmd := &cobra.Command{
		Use:   "run <task-path>",
		Short: "Run a coding agent against a task file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.TaskPath = args[0]

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

			containerName := run.ContainerName(runID)

			r := &runner.Runner{
				RunID:       runID,
				EvidenceDir: evidenceDir,
				Config:      cfg,
			}
			if err := r.Run(cmd.Context()); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "run_id: %s\n", runID)
			fmt.Fprintf(cmd.OutOrStdout(), "evidence_path: %s\n", evidenceDir)
			fmt.Fprintf(cmd.OutOrStdout(), "workspace_path: %s\n", wsPath)
			fmt.Fprintf(cmd.OutOrStdout(), "container_name: %s\n", containerName)
			fmt.Fprintf(cmd.OutOrStdout(), "attach: tessariq attach %s\n", runID)
			fmt.Fprintf(cmd.OutOrStdout(), "promote: tessariq promote %s\n", runID)

			return nil
		},
	}

	cmd.Flags().DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "maximum run duration")
	cmd.Flags().DurationVar(&cfg.Grace, "grace", cfg.Grace, "grace period after timeout before kill")
	cmd.Flags().StringVar(&cfg.Agent, "agent", cfg.Agent, "agent adapter (claude-code|opencode)")
	cmd.Flags().StringVar(&cfg.Image, "image", cfg.Image, "container image override")
	cmd.Flags().StringVar(&cfg.Model, "model", cfg.Model, "model identifier for the agent")
	cmd.Flags().BoolVar(&cfg.Yolo, "yolo", cfg.Yolo, "enable yolo mode for the agent")
	cmd.Flags().StringVar(&cfg.Egress, "egress", cfg.Egress, "egress mode (none|proxy|open|auto)")
	cmd.Flags().BoolVar(&cfg.UnsafeEgress, "unsafe-egress", cfg.UnsafeEgress, "alias for --egress open")
	cmd.Flags().StringArrayVar(&cfg.EgressAllow, "egress-allow", cfg.EgressAllow, "allowed egress destination (repeatable)")
	cmd.Flags().BoolVar(&cfg.EgressAllowReset, "egress-allow-reset", cfg.EgressAllowReset, "discard built-in and user-configured allowlist")
	cmd.Flags().StringArrayVar(&cfg.Pre, "pre", cfg.Pre, "pre-command to run before the agent (repeatable)")
	cmd.Flags().StringArrayVar(&cfg.Verify, "verify", cfg.Verify, "verify command to run after the agent (repeatable)")
	cmd.Flags().BoolVar(&cfg.Attach, "attach", cfg.Attach, "attach to the run session immediately")

	return cmd
}
