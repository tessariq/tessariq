package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/prereq"
	intpromote "github.com/tessariq/tessariq/internal/promote"
)

type promoteOutput struct {
	Branch string
	Commit string
}

func printPromoteOutput(w io.Writer, out promoteOutput) {
	fmt.Fprintf(w, "branch: %s\n", out.Branch)
	fmt.Fprintf(w, "commit: %s\n", out.Commit)
}

func newPromoteCmd() *cobra.Command {
	var opts intpromote.Options

	cmd := &cobra.Command{
		Use:   "promote <run-ref>",
		Short: "Promote a finished run into one branch and one commit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prereq.NewChecker().CheckCommand("promote"); err != nil {
				return err
			}

			root, err := repoRoot(cmd)
			if err != nil {
				return err
			}

			opts.RunRef = args[0]
			result, err := intpromote.Run(cmd.Context(), root, opts)
			if err != nil {
				return err
			}

			printPromoteOutput(cmd.OutOrStdout(), promoteOutput{
				Branch: result.Branch,
				Commit: result.Commit,
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.Branch, "branch", "", "branch name for the promoted commit")
	cmd.Flags().StringVar(&opts.Message, "message", "", "commit message for the promoted commit")
	cmd.Flags().BoolVar(&opts.NoTrailers, "no-trailers", false, "suppress default Tessariq commit trailers")

	return cmd
}
