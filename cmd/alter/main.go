package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/iomz/alter/internal/plugin"
	"github.com/iomz/alter/internal/runtime"
	"github.com/iomz/alter/internal/ui"
	cli "github.com/urfave/cli/v3"
)

func main() {
	if err := newCommand().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "alter:", err)
		os.Exit(1)
	}
}

func newCommand() *cli.Command {
	return &cli.Command{
		Name:  "alter",
		Usage: "local/private tool control plane",
		Commands: []*cli.Command{
			newPluginCommand(),
			newSetupCommand(),
		},
	}
}

func newPluginCommand() *cli.Command {
	return &cli.Command{
		Name:  "plugin",
		Usage: "inspect and check plugins",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list local plugins",
				Action: func(context.Context, *cli.Command) error {
					store, err := pluginContext()
					if err != nil {
						return err
					}
					plugins, err := store.List()
					if err != nil {
						return err
					}
					for _, p := range plugins {
						fmt.Printf("%s\t%s\n", p.Manifest.Plugin.Name, p.Manifest.Plugin.Description)
					}
					return nil
				},
			},
			{
				Name:      "inspect",
				Usage:     "print plugin manifest",
				ArgsUsage: "<name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					if cmd.NArg() != 1 {
						return errors.New("usage: alter plugin inspect <name>")
					}
					store, err := pluginContext()
					if err != nil {
						return err
					}
					p, err := store.Load(cmd.Args().First())
					if err != nil {
						return err
					}
					return printJSON(p.Manifest)
				},
			},
			{
				Name:      "doctor",
				Usage:     "validate plugin manifest and layout",
				ArgsUsage: "<name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					if cmd.NArg() != 1 {
						return errors.New("usage: alter plugin doctor <name>")
					}
					store, err := pluginContext()
					if err != nil {
						return err
					}
					report, err := store.Doctor(cmd.Args().First())
					if err != nil {
						return err
					}
					return printJSON(report)
				},
			},
		},
	}
}

func newSetupCommand() *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "inspect local alter setup",
		Commands: []*cli.Command{
			{
				Name:  "mise",
				Usage: "inspect mise runtime discovery",
				Action: func(ctx context.Context, _ *cli.Command) error {
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
					installedPath, err := runtime.NewMiseBootstrapper(os.Stdout, os.Stderr).Install(ctx)
					if err != nil {
						return err
					}
					ui.PrintRuntimeInstalled(os.Stdout, installedPath)
					return nil
				},
			},
			{
				Name:  "shell",
				Usage: "inspect shell integration setup",
				Action: func(context.Context, *cli.Command) error {
					err := ui.PrintStub(os.Stdout, "setup shell", "Shell integration is not implemented in Phase 1. alter does not modify shell startup files.")
					ui.PrintPromptDeferred(os.Stdout)
					return err
				},
			},
		},
	}
}

func pluginContext() (*plugin.Store, error) {
	root, err := plugin.FindRepoRoot()
	if err != nil {
		return nil, err
	}
	return plugin.NewStore(root), nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
