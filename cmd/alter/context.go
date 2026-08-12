package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/iomz/alter/internal/adapter"
	"github.com/iomz/alter/internal/plugin"
	"github.com/iomz/alter/internal/runtime"
)

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
