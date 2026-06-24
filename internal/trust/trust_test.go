package trust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iomz/alter/internal/plugin"
)

func TestTrustEvaluateAndInvalidateOnFileChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := writePlugin(t, home, "test-runtime")

	eval, err := Evaluate(p)
	if err != nil {
		t.Fatal(err)
	}
	if eval.Status != StatusUntrusted {
		t.Fatalf("status = %s, want untrusted", eval.Status)
	}

	record, storePath, err := Trust(p)
	if err != nil {
		t.Fatal(err)
	}
	if record.Name != "test-runtime" {
		t.Fatalf("record name = %q, want test-runtime", record.Name)
	}
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("trust store missing: %v", err)
	}

	eval, err = Evaluate(p)
	if err != nil {
		t.Fatal(err)
	}
	if eval.Status != StatusTrusted {
		t.Fatalf("status = %s, want trusted", eval.Status)
	}

	if err := os.WriteFile(filepath.Join(p.Path, "alter-test-runtime"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	eval, err = Evaluate(p)
	if err != nil {
		t.Fatal(err)
	}
	if eval.Status != StatusInvalidated {
		t.Fatalf("status = %s, want invalidated", eval.Status)
	}
	if len(eval.Mismatches) == 0 {
		t.Fatal("mismatches empty, want entrypoint mismatch")
	}
}

func TestUntrustRemovesRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := writePlugin(t, home, "test-runtime")
	if _, _, err := Trust(p); err != nil {
		t.Fatal(err)
	}
	removed, _, err := Untrust("test-runtime")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("removed = false, want true")
	}
	eval, err := Evaluate(p)
	if err != nil {
		t.Fatal(err)
	}
	if eval.Status != StatusUntrusted {
		t.Fatalf("status = %s, want untrusted", eval.Status)
	}
}

func writePlugin(t *testing.T, root, name string) plugin.Plugin {
	t.Helper()
	dir := filepath.Join(root, "plugins", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[plugin]
name = "` + name + `"
description = "Mise test"
maintainer = "iomz"
entrypoint = "alter-test-runtime"

[upstream]
name = "` + name + `"
repository = ""

[runtime]
manager = "mise"

[mcp]
enabled = false
namespace = ""
`
	if err := os.WriteFile(filepath.Join(dir, plugin.ManifestFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alter.mise.toml"), []byte("[tools]\nnode = \"24\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alter-test-runtime"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := plugin.NewStore(root).Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
