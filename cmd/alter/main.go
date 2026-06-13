package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/iomz/alter/internal/mcp"
	"github.com/iomz/alter/internal/plugin"
	"github.com/iomz/alter/internal/runtime"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "alter:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	root, err := plugin.FindRepoRoot()
	if err != nil {
		return err
	}
	store := plugin.NewStore(root)
	runner := runtime.NewMiseRunner(os.Stdout, os.Stderr)

	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "plugin":
		return runPlugin(store, runner, args[1:])
	case "mcp":
		if len(args) != 1 {
			return errors.New("usage: alter mcp")
		}
		return mcp.Serve(os.Stdin, os.Stdout, store, runner)
	case "hello":
		return runHello(store, runner, args[1:])
	default:
		return usage()
	}
}

func runPlugin(store *plugin.Store, runner *runtime.MiseRunner, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: alter plugin <list|inspect|doctor>")
	}
	switch args[0] {
	case "list":
		plugins, err := store.List()
		if err != nil {
			return err
		}
		for _, p := range plugins {
			fmt.Printf("%s\t%s\n", p.Manifest.Plugin.Name, p.Manifest.Plugin.Description)
		}
		return nil
	case "inspect":
		if len(args) != 2 {
			return errors.New("usage: alter plugin inspect <name>")
		}
		p, err := store.Load(args[1])
		if err != nil {
			return err
		}
		return printJSON(p.Manifest)
	case "doctor":
		if len(args) != 2 {
			return errors.New("usage: alter plugin doctor <name>")
		}
		p, err := store.Load(args[1])
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
	default:
		return errors.New("usage: alter plugin <list|inspect|doctor>")
	}
}

func runHello(store *plugin.Store, runner *runtime.MiseRunner, args []string) error {
	if len(args) == 0 || args[0] != "greet" {
		return errors.New("usage: alter hello greet --name <name>")
	}
	name := "world"
	for i := 1; i < len(args); i++ {
		if args[i] == "--name" && i+1 < len(args) {
			name = args[i+1]
			i++
			continue
		}
		return errors.New("usage: alter hello greet --name <name>")
	}

	p, err := store.Load("hello")
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"tool": "greet",
		"args": map[string]any{"name": name},
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
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func usage() error {
	return errors.New("usage: alter <plugin|hello|mcp>")
}
