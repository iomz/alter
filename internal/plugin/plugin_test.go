package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreListLoadsTypedManifestsSortedByName(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "sample", "Sample adapter boundary", "alter-sample")
	writeManifest(t, root, "ingest", "Adapter boundary for ingest tooling", "alter-ingest")

	store := NewStore(root)
	got, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("List() length = %d, want 2", len(got))
	}
	if got[0].Manifest.Plugin.Name != "ingest" {
		t.Fatalf("first plugin = %q, want ingest", got[0].Manifest.Plugin.Name)
	}
	if got[1].Manifest.Plugin.Name != "sample" {
		t.Fatalf("second plugin = %q, want sample", got[1].Manifest.Plugin.Name)
	}
}

func TestStoreLoadRejectsManifestNameMismatch(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "hello", "Example alter plugin", "alter-hello")

	manifestPath := filepath.Join(root, "plugins", "hello", ManifestFileName)
	body := []byte(`[plugin]
name = "wrong"
description = "Wrong plugin"
maintainer = "iomz"
entrypoint = "alter-wrong"

[upstream]
name = "wrong"
repository = ""

[runtime]
manager = "mise"

[mcp]
enabled = false
namespace = ""
`)
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := NewStore(root).Load("hello")
	if err == nil {
		t.Fatal("Load() error = nil, want mismatch error")
	}
}

func TestStoreLoadRejectsEntrypointOutsideWorkspace(t *testing.T) {
	for _, entrypoint := range []string{
		"../outside",
		filepath.Join(string(filepath.Separator), "tmp", "outside"),
	} {
		t.Run(entrypoint, func(t *testing.T) {
			root := t.TempDir()
			writeManifest(t, root, "hello", "Example alter plugin", entrypoint)

			_, err := NewStore(root).Load("hello")
			if err == nil {
				t.Fatal("Load() error = nil, want entrypoint workspace error")
			}
		})
	}
}

func TestStoreLoadAcceptsNestedEntrypointInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "hello", "Example alter plugin", filepath.Join("bin", "alter-hello"))

	if _, err := NewStore(root).Load("hello"); err != nil {
		t.Fatalf("Load() error = %v, want nested entrypoint accepted", err)
	}
}

func TestStoreDoctorDoesNotRequireEntrypointExecution(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "ingest", "Adapter boundary for ingest tooling", "alter-ingest")

	report, err := NewStore(root).Doctor("ingest")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "ok" {
		t.Fatalf("Doctor() status = %q, want ok", report.Status)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("Doctor() warnings = %#v, want missing entrypoint warning", report.Warnings)
	}
}

func writeManifest(t *testing.T, root, name, description, entrypoint string) {
	t.Helper()
	dir := filepath.Join(root, "plugins", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `[plugin]
name = "` + name + `"
description = "` + description + `"
maintainer = "iomz"
entrypoint = "` + entrypoint + `"

[upstream]
name = "` + name + `"
repository = ""

[runtime]
manager = "mise"

[mcp]
enabled = false
namespace = ""
`
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
