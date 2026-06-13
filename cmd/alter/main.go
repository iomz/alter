package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

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
			newMCPCommand(),
			newHelloCommand(),
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
					store, _, err := runtimeContext()
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
					store, _, err := runtimeContext()
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
				Usage:     "prepare and run plugin doctor",
				ArgsUsage: "<name>",
				Action: func(_ context.Context, cmd *cli.Command) error {
					if cmd.NArg() != 1 {
						return errors.New("usage: alter plugin doctor <name>")
					}
					store, runner, err := runtimeContext()
					if err != nil {
						return err
					}
					p, err := store.Load(cmd.Args().First())
					if err != nil {
						return err
					}
					if err := runner.Prepare(p); err != nil {
						return err
					}
					out, err := runner.Run(p, "doctor")
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
				Action: func(context.Context, *cli.Command) error {
					resolver := runtime.NewMiseResolver()
					path, err := resolver.Resolve()
					if err != nil {
						ui.PrintRuntimeMissing(os.Stdout, err)
						return ui.PrintStub(os.Stdout, "setup mise", "Installation is not implemented in Phase 1. Runtime discovery only checks PATH and alter-managed locations.")
					}
					ui.PrintRuntimeFound(os.Stdout, path)
					return ui.PrintStub(os.Stdout, "setup mise", "Installation is not implemented in Phase 1. Runtime discovery only checks PATH and alter-managed locations.")
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

func newMCPCommand() *cli.Command {
	return &cli.Command{
		Name:  "mcp",
		Usage: "serve MCP over stdio",
		Action: func(context.Context, *cli.Command) error {
			store, runner, err := runtimeContext()
			if err != nil {
				return err
			}
			return mcp.Serve(os.Stdin, os.Stdout, store, runner)
		},
	}
}

func newHelloCommand() *cli.Command {
	return &cli.Command{
		Name:  "hello",
		Usage: "run hello prototype plugin",
		Commands: []*cli.Command{
			{
				Name:  "greet",
				Usage: "print greeting",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Value: "world", Usage: "name to greet"},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					store, runner, err := runtimeContext()
					if err != nil {
						return err
					}
					p, err := store.Load("hello")
					if err != nil {
						return err
					}
					payload, err := json.Marshal(map[string]any{
						"tool": "greet",
						"args": map[string]any{"name": cmd.String("name")},
					})
					if err != nil {
						return err
					}
					out, err := runner.Run(p, "invoke", string(payload))
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

func runtimeContext() (*plugin.Store, *runtime.MiseRunner, error) {
	root, err := plugin.FindRepoRoot()
	if err != nil {
		return nil, nil, err
	}
	store := plugin.NewStore(root)
	runner := runtime.NewMiseRunner(os.Stdout, os.Stderr)
	return store, runner, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
