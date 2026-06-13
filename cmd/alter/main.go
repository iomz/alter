package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/iomz/alter/internal/adapter"
	"github.com/iomz/alter/internal/mcp"
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
			newHelloCommand(),
			newMCPCommand(),
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
					name := cmd.Args().First()
					report, err := store.Doctor(name)
					if err != nil {
						return err
					}
					if len(report.Warnings) > 0 {
						return printJSON(report)
					}
					executor, err := executorContext()
					if err != nil {
						return err
					}
					out, err := executor.Doctor(name)
					if err != nil {
						return err
					}
					fmt.Print(string(out))
					return nil
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
			{
				Name:  "cleanup",
				Usage: "remove alter-managed mise runtime files",
				Action: func(context.Context, *cli.Command) error {
					items, err := runtime.CleanupManagedMise()
					if err != nil {
						return err
					}
					ui.PrintCleanupReport(os.Stdout, items)
					return nil
				},
			},
		},
	}
}

func newHelloCommand() *cli.Command {
	return &cli.Command{
		Name:  "hello",
		Usage: "run hello adapter",
		Commands: []*cli.Command{
			{
				Name:  "greet",
				Usage: "return greeting JSON",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Value: "world", Usage: "name to greet"},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					executor, err := executorContext()
					if err != nil {
						return err
					}
					out, err := executor.Invoke("hello", "greet", map[string]any{"name": cmd.String("name")})
					if err != nil {
						return err
					}
					fmt.Print(string(out))
					return nil
				},
			},
		},
	}
}

func newMCPCommand() *cli.Command {
	return &cli.Command{
		Name:  "mcp",
		Usage: "serve MCP over stdio",
		Action: func(ctx context.Context, _ *cli.Command) error {
			executor, err := executorContextWithRuntimeOutput(io.Discard, os.Stderr)
			if err != nil {
				return err
			}
			return mcp.Serve(ctx, executor)
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

func executorContext() (*adapter.Executor, error) {
	return executorContextWithRuntimeOutput(os.Stdout, os.Stderr)
}

func executorContextWithRuntimeOutput(stdout, stderr io.Writer) (*adapter.Executor, error) {
	store, err := pluginContext()
	if err != nil {
		return nil, err
	}
	return adapter.NewExecutor(store, runtime.NewMiseRunner(stdout, stderr)), nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
