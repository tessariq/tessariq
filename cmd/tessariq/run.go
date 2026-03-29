package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/run"
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

			runID, evidenceDir, err := run.BootstrapManifest(root, cfg, time.Now())
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "run_id: %s\n", runID)
			fmt.Fprintf(cmd.OutOrStdout(), "evidence_path: %s\n", evidenceDir)

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
