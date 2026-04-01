package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/version"
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "tessariq",
		Short:         "Git-native, sandboxed CLI for running coding agents against repositories",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
	}

	cmd.SetVersionTemplate("{{.Name}} v{{.Version}}\n")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newAttachCmd())
	cmd.AddCommand(newPromoteCmd())

	return cmd
}
