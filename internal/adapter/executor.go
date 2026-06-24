package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/iomz/alter/internal/plugin"
)

type Runtime interface {
	PrepareFingerprint(plugin.Plugin) (string, error)
	Prepare(context.Context, plugin.Plugin) error
	Run(context.Context, plugin.Plugin, ...string) ([]byte, error)
}

type Executor struct {
	store            *plugin.Store
	runtime          Runtime
	prepareMu        sync.Mutex
	preparedByPlugin map[string]string
}

func NewExecutor(store *plugin.Store, runtime Runtime) *Executor {
	return &Executor{
		store:            store,
		runtime:          runtime,
		preparedByPlugin: make(map[string]string),
	}
}

func (e *Executor) Manifest(ctx context.Context, name string) ([]byte, error) {
	p, err := e.store.Load(name)
	if err != nil {
		return nil, err
	}
	return e.run(ctx, p, "manifest")
}

func (e *Executor) Doctor(ctx context.Context, name string) ([]byte, error) {
	p, err := e.store.Load(name)
	if err != nil {
		return nil, err
	}
	return e.run(ctx, p, "doctor")
}

func (e *Executor) Invoke(ctx context.Context, name, tool string, args any) ([]byte, error) {
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
	return e.run(ctx, p, "invoke", string(payload))
}

func (e *Executor) run(ctx context.Context, p plugin.Plugin, args ...string) ([]byte, error) {
	if err := e.prepare(ctx, p); err != nil {
		return nil, err
	}
	out, err := e.runtime.Run(ctx, p, args...)
	if err != nil {
		return nil, err
	}
	return NormalizeJSON(out)
}

func (e *Executor) prepare(ctx context.Context, p plugin.Plugin) error {
	fingerprint, err := e.runtime.PrepareFingerprint(p)
	if err != nil {
		return err
	}

	e.prepareMu.Lock()
	defer e.prepareMu.Unlock()

	if cached, ok := e.preparedByPlugin[p.Path]; ok && cached == fingerprint {
		return nil
	}
	if err := e.runtime.Prepare(ctx, p); err != nil {
		return err
	}
	e.preparedByPlugin[p.Path] = fingerprint
	return nil
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
