package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/initialize"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize tessariq runtime state in the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := repoRoot(cmd)
			if err != nil {
				return err
			}
			if err := initialize.Run(root); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "initialized", root)
			return nil
		},
	}
}

func repoRoot(cmd *cobra.Command) (string, error) {
	out, err := exec.CommandContext(cmd.Context(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
