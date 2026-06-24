package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/iomz/alter/internal/plugin"
)

func TestExecutorInvokePreparesRunsAndNormalizesOutput(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "hello")
	runtime := &fakeRuntime{out: []byte(`{"message":"hello, iomz","plugin":"hello"}`)}
	executor := NewExecutor(plugin.NewStore(root), runtime)

	got, err := executor.Invoke(context.Background(), "hello", "greet", map[string]any{"name": "iomz"})
	if err != nil {
		t.Fatal(err)
	}
	if len(runtime.prepared) != 1 || runtime.prepared[0] != "hello" {
		t.Fatalf("prepared = %#v, want [hello]", runtime.prepared)
	}
	if len(runtime.runs) != 1 {
		t.Fatalf("runs = %#v, want one run", runtime.runs)
	}
	run := runtime.runs[0]
	if run.name != "hello" {
		t.Fatalf("run plugin = %q, want hello", run.name)
	}
	if len(run.args) != 2 || run.args[0] != "invoke" {
		t.Fatalf("run args = %#v, want invoke payload", run.args)
	}
	var payload struct {
		Tool string         `json:"tool"`
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(run.args[1]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Tool != "greet" || payload.Args["name"] != "iomz" {
		t.Fatalf("payload = %#v, want greet iomz", payload)
	}

	want := "{\n  \"message\": \"hello, iomz\",\n  \"plugin\": \"hello\"\n}\n"
	if string(got) != want {
		t.Fatalf("Invoke() = %q, want %q", got, want)
	}
}

func TestExecutorCachesPrepareForSameFingerprint(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "hello")
	runtime := &fakeRuntime{
		fingerprint: "fingerprint-1",
		out:         []byte(`{"ok":true}`),
	}
	executor := NewExecutor(plugin.NewStore(root), runtime)

	for range 2 {
		if _, err := executor.Invoke(context.Background(), "hello", "greet", map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(runtime.prepared) != 1 {
		t.Fatalf("prepared = %#v, want one prepare", runtime.prepared)
	}
	if len(runtime.runs) != 2 {
		t.Fatalf("runs = %#v, want two runs", runtime.runs)
	}
}

func TestExecutorRepreparesWhenFingerprintChanges(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "hello")
	runtime := &fakeRuntime{
		fingerprint: "fingerprint-1",
		out:         []byte(`{"ok":true}`),
	}
	executor := NewExecutor(plugin.NewStore(root), runtime)

	if _, err := executor.Invoke(context.Background(), "hello", "greet", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	runtime.fingerprint = "fingerprint-2"
	if _, err := executor.Invoke(context.Background(), "hello", "greet", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if len(runtime.prepared) != 2 {
		t.Fatalf("prepared = %#v, want prepare after fingerprint change", runtime.prepared)
	}
}

func TestExecutorDoesNotCacheFailedPrepare(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "hello")
	runtime := &fakeRuntime{
		fingerprint: "fingerprint-1",
		prepareErr:  errors.New("prepare failed"),
		out:         []byte(`{"ok":true}`),
	}
	executor := NewExecutor(plugin.NewStore(root), runtime)

	if _, err := executor.Invoke(context.Background(), "hello", "greet", map[string]any{}); err == nil {
		t.Fatal("Invoke() error = nil, want prepare error")
	}
	runtime.prepareErr = nil
	if _, err := executor.Invoke(context.Background(), "hello", "greet", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if len(runtime.prepared) != 2 {
		t.Fatalf("prepared = %#v, want failed prepare retried", runtime.prepared)
	}
}

func TestNormalizeJSONRejectsInvalidOutput(t *testing.T) {
	_, err := NormalizeJSON([]byte("not json"))
	if err == nil {
		t.Fatal("NormalizeJSON() error = nil, want invalid JSON error")
	}
}

type fakeRuntime struct {
	fingerprint string
	prepareErr  error
	out         []byte
	prepared    []string
	runs        []fakeRun
}

type fakeRun struct {
	name string
	args []string
}

func (r *fakeRuntime) PrepareFingerprint(plugin.Plugin) (string, error) {
	if r.fingerprint == "" {
		return "default", nil
	}
	return r.fingerprint, nil
}

func (r *fakeRuntime) Prepare(_ context.Context, p plugin.Plugin) error {
	r.prepared = append(r.prepared, p.Manifest.Plugin.Name)
	return r.prepareErr
}

func (r *fakeRuntime) Run(_ context.Context, p plugin.Plugin, args ...string) ([]byte, error) {
	r.runs = append(r.runs, fakeRun{name: p.Manifest.Plugin.Name, args: args})
	return r.out, nil
}

func writeManifest(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, "plugins", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `[plugin]
name = "` + name + `"
description = "Example plugin"
maintainer = "iomz"
entrypoint = "alter-` + name + `"

[upstream]
name = "` + name + `"
repository = ""

[runtime]
manager = "mise"

[mcp]
enabled = false
namespace = ""
`
	if err := os.WriteFile(filepath.Join(dir, plugin.ManifestFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
