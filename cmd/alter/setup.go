package main

import (
	"fmt"
	"os"

	"github.com/iomz/alter/internal/runtime"
	"github.com/iomz/alter/internal/ui"
	"github.com/spf13/cobra"
)

func newSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "inspect local alter setup",
	}
	cmd.AddCommand(
		newSetupMiseCommand(),
		newSetupShellCommand(),
		newSetupCleanupCommand(),
	)
	return cmd
}

func newSetupMiseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mise",
		Short: "inspect mise runtime discovery",
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolver := runtime.NewMiseResolver()
			path, err := resolver.Resolve()
			if err == nil {
				ui.PrintRuntimeFound(os.Stdout, path)
				return nil
			}
			ui.PrintRuntimeMissing(os.Stdout, err)
			installPath, pathErr := resolver.ManagedInstallPath()
			if pathErr != nil {
				return pathErr
			}
			if err := ui.PrintMiseBootstrapExplanation(os.Stdout, installPath); err != nil {
				return err
			}
			confirmed, err := ui.ConfirmMiseBootstrap(os.Stdout, os.Stdin)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Fprintln(os.Stdout, ui.Warning("cancelled"), "mise installation skipped")
				return nil
			}
			installedPath, err := runtime.NewMiseBootstrapper(os.Stdout, os.Stderr).Install(cmd.Context())
			if err != nil {
				return err
			}
			ui.PrintRuntimeInstalled(os.Stdout, installedPath)
			return nil
		},
	}
}

func newSetupShellCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "inspect shell integration setup",
		RunE: func(*cobra.Command, []string) error {
			err := ui.PrintStub(os.Stdout, "setup shell", "Shell integration is not implemented in Phase 1. alter does not modify shell startup files.")
			ui.PrintPromptDeferred(os.Stdout)
			return err
		},
	}
}

func newSetupCleanupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "remove alter-managed mise runtime files",
		RunE: func(*cobra.Command, []string) error {
			items, err := runtime.CleanupManagedMise()
			if err != nil {
				return err
			}
			ui.PrintCleanupReport(os.Stdout, items)
			return nil
		},
	}
}
