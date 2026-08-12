package main

import (
	"fmt"
	"os"

	"github.com/iomz/alter/internal/plugin"
	"github.com/iomz/alter/internal/runtime"
	"github.com/iomz/alter/internal/trust"
	"github.com/iomz/alter/internal/ui"
	"github.com/spf13/cobra"
)

func newPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "inspect and check plugins",
	}
	cmd.AddCommand(
		newPluginListCommand(),
		newPluginInspectCommand(),
		newPluginDoctorCommand(),
		newPluginTrustCommand(),
		newPluginUntrustCommand(),
		newPluginTrustStatusCommand(),
	)
	return cmd
}

func newPluginListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list local plugins",
		RunE: func(*cobra.Command, []string) error {
			store, err := pluginContext()
			if err != nil {
				return err
			}
			plugins, err := store.List()
			if err != nil {
				return err
			}
			ui.PrintPluginList(os.Stdout, plugins)
			return nil
		},
	}
}

func newPluginInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <name>",
		Short: "print plugin manifest",
		Args:  exactArgs("usage: alter plugin inspect <name>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := pluginContext()
			if err != nil {
				return err
			}
			p, err := store.Load(args[0])
			if err != nil {
				return err
			}
			jsonOutput, err := cmd.Flags().GetBool("json")
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(p.Manifest)
			}
			ui.PrintPluginManifest(os.Stdout, p)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "print raw manifest JSON")
	return cmd
}

func newPluginDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor <name>",
		Short: "validate plugin manifest and layout",
		Args:  exactArgs("usage: alter plugin doctor <name>"),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := pluginContext()
			if err != nil {
				return err
			}
			report, err := store.Doctor(args[0])
			if err != nil {
				return err
			}
			if len(report.Warnings) > 0 {
				ui.PrintPluginDoctorReport(os.Stdout, report)
				return nil
			}
			p, err := store.Load(args[0])
			if err != nil {
				return err
			}
			runner := runtime.NewMiseRunner(os.Stdout, os.Stderr)
			diagnostics, err := runner.Diagnostics(p)
			if err != nil {
				return err
			}
			ui.PrintRuntimeDiagnostics(os.Stdout, diagnostics)
			return nil
		},
	}
}

func newPluginTrustCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "trust <name>",
		Short: "trust current plugin runtime fingerprints",
		Args:  exactArgs("usage: alter plugin trust <name>"),
		RunE: func(_ *cobra.Command, args []string) error {
			p, diagnostics, err := pluginDiagnostics(args[0])
			if err != nil {
				return err
			}
			if diagnostics.RuntimeMode == runtime.RuntimeModeDirect && diagnostics.InstallSkipped {
				ui.PrintTrustNotRequired(os.Stdout, p)
				return nil
			}
			ui.PrintTrustReview(os.Stdout, p, diagnostics)
			confirmed, err := ui.ConfirmPluginTrust(os.Stdout, os.Stdin, p.Manifest.Plugin.Name)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Fprintln(os.Stdout, ui.Warning("cancelled"), "plugin trust unchanged")
				return nil
			}
			record, storePath, err := trust.Trust(p)
			if err != nil {
				return err
			}
			ui.PrintTrustSaved(os.Stdout, record, storePath)
			return nil
		},
	}
}

func newPluginUntrustCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "untrust <name>",
		Short: "remove plugin trust record",
		Args:  exactArgs("usage: alter plugin untrust <name>"),
		RunE: func(_ *cobra.Command, args []string) error {
			removed, storePath, err := trust.Untrust(args[0])
			if err != nil {
				return err
			}
			ui.PrintTrustRemoved(os.Stdout, args[0], storePath, removed)
			return nil
		},
	}
}

func newPluginTrustStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "trust-status <name>",
		Short: "show plugin trust status",
		Args:  exactArgs("usage: alter plugin trust-status <name>"),
		RunE: func(_ *cobra.Command, args []string) error {
			_, diagnostics, err := pluginDiagnostics(args[0])
			if err != nil {
				return err
			}
			ui.PrintRuntimeDiagnostics(os.Stdout, diagnostics)
			return nil
		},
	}
}

func pluginDiagnostics(name string) (plugin.Plugin, runtime.DiagnosticReport, error) {
	store, err := pluginContext()
	if err != nil {
		return plugin.Plugin{}, runtime.DiagnosticReport{}, err
	}
	p, err := store.Load(name)
	if err != nil {
		return plugin.Plugin{}, runtime.DiagnosticReport{}, err
	}
	runner := runtime.NewMiseRunner(os.Stdout, os.Stderr)
	diagnostics, err := runner.Diagnostics(p)
	if err != nil {
		return plugin.Plugin{}, runtime.DiagnosticReport{}, err
	}
	return p, diagnostics, nil
}
