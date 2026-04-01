package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	intattach "github.com/tessariq/tessariq/internal/attach"
	"github.com/tessariq/tessariq/internal/prereq"
	"github.com/tessariq/tessariq/internal/tmux"
)

var (
	checkAttachPrereq = func() error {
		return prereq.NewChecker().CheckCommand("attach")
	}
	attachRepoRoot = func(cmd *cobra.Command) (string, error) {
		return repoRoot(cmd)
	}
	resolveAttachRun = func(ctx context.Context, repoRoot, ref string) (intattach.Result, error) {
		return intattach.ResolveLiveRun(ctx, repoRoot, ref)
	}
	attachToSession = func(ctx context.Context, sessionName string) error {
		return tmux.AttachSession(ctx, sessionName)
	}
)

func newAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach <run-ref>",
		Short: "Attach to a live run tmux session",
		Long:  "Attach your terminal to a live run's tmux session.\n\nDetach without stopping the run with Ctrl-b d.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkAttachPrereq(); err != nil {
				return err
			}

			root, err := attachRepoRoot(cmd)
			if err != nil {
				return err
			}

			result, err := resolveAttachRun(cmd.Context(), root, args[0])
			if err != nil {
				return err
			}

			if err := attachToSession(cmd.Context(), result.SessionName); err != nil {
				return fmt.Errorf("attach run %s: %w", result.RunID, err)
			}
			return nil
		},
	}

	return cmd
}
