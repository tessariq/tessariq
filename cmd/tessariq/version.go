package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Tessariq version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "tessariq v%s\n", version.Version)
			return err
		},
	}
}
