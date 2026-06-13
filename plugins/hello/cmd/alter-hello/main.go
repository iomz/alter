package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "alter-hello:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: alter-hello <manifest|doctor|invoke>")
	}
	switch args[0] {
	case "manifest":
		return printJSON(map[string]any{
			"name":        "hello",
			"description": "Example alter plugin",
			"tools": []map[string]any{{
				"name":        "greet",
				"description": "Return predictable greeting JSON",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"name": map[string]any{"type": "string"}},
					"required":   []string{"name"},
				},
			}},
		})
	case "doctor":
		return printJSON(map[string]any{"ok": true, "plugin": "hello"})
	case "invoke":
		if len(args) != 2 {
			return errors.New("usage: alter-hello invoke <json>")
		}
		return invoke(args[1])
	default:
		return errors.New("usage: alter-hello <manifest|doctor|invoke>")
	}
}

func invoke(raw string) error {
	var input struct {
		Tool string `json:"tool"`
		Args struct {
			Name string `json:"name"`
		} `json:"args"`
	}
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return err
	}
	if input.Tool != "greet" {
		return fmt.Errorf("unknown tool %q", input.Tool)
	}
	name := input.Args.Name
	if name == "" {
		name = "world"
	}
	return printJSON(map[string]any{
		"message": fmt.Sprintf("hello, %s", name),
		"plugin":  "hello",
	})
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
