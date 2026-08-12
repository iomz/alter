package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "alter",
		Short:         "local/private tool control plane",
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	cmd.AddCommand(
		newPluginCommand(),
		newSetupCommand(),
		newHelloCommand(),
		newTestRuntimeCommand(),
		newMCPCommand(),
	)
	return cmd
}

func exactArgs(usage string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New(usage)
		}
		return nil
	}
}
