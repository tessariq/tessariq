package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tessariq/tessariq/internal/runner"
	"github.com/tessariq/tessariq/internal/version"
)

func main() {
	ctx, stop := newSignalContext()
	defer stop()

	cmd := newRootCmd()
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newSignalContext() (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig, ok := <-sigCh
		if !ok {
			return
		}
		cancel(runner.SignalCause(sig))
	}()

	return ctx, func() {
		signal.Stop(sigCh)
		close(sigCh)
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
