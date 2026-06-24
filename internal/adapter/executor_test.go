package adapter

import (
	"encoding/json"
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

	got, err := executor.Invoke("hello", "greet", map[string]any{"name": "iomz"})
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

func TestNormalizeJSONRejectsInvalidOutput(t *testing.T) {
	_, err := NormalizeJSON([]byte("not json"))
	if err == nil {
		t.Fatal("NormalizeJSON() error = nil, want invalid JSON error")
	}
}

type fakeRuntime struct {
	out      []byte
	prepared []string
	runs     []fakeRun
}

type fakeRun struct {
	name string
	args []string
}

func (r *fakeRuntime) Prepare(p plugin.Plugin) error {
	r.prepared = append(r.prepared, p.Manifest.Plugin.Name)
	return nil
}

func (r *fakeRuntime) Run(p plugin.Plugin, args ...string) ([]byte, error) {
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
