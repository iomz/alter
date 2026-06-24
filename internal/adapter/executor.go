package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/iomz/alter/internal/plugin"
)

type Runtime interface {
	Prepare(plugin.Plugin) error
	Run(plugin.Plugin, ...string) ([]byte, error)
}

type Executor struct {
	store   *plugin.Store
	runtime Runtime
}

func NewExecutor(store *plugin.Store, runtime Runtime) *Executor {
	return &Executor{store: store, runtime: runtime}
}

func (e *Executor) Manifest(name string) ([]byte, error) {
	p, err := e.store.Load(name)
	if err != nil {
		return nil, err
	}
	return e.run(p, "manifest")
}

func (e *Executor) Doctor(name string) ([]byte, error) {
	p, err := e.store.Load(name)
	if err != nil {
		return nil, err
	}
	return e.run(p, "doctor")
}

func (e *Executor) Invoke(name, tool string, args any) ([]byte, error) {
	p, err := e.store.Load(name)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"tool": tool,
		"args": args,
	})
	if err != nil {
		return nil, fmt.Errorf("encode adapter invocation: %w", err)
	}
	return e.run(p, "invoke", string(payload))
}

func (e *Executor) run(p plugin.Plugin, args ...string) ([]byte, error) {
	if err := e.runtime.Prepare(p); err != nil {
		return nil, err
	}
	out, err := e.runtime.Run(p, args...)
	if err != nil {
		return nil, err
	}
	return NormalizeJSON(out)
}

func NormalizeJSON(out []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("adapter returned empty output")
	}
	var normalized bytes.Buffer
	if err := json.Indent(&normalized, trimmed, "", "  "); err != nil {
		return nil, fmt.Errorf("adapter returned invalid JSON: %w", err)
	}
	normalized.WriteByte('\n')
	return normalized.Bytes(), nil
}
