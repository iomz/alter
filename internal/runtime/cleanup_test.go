package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupManagedMiseRemovesOnlyAlterManagedTargets(t *testing.T) {
	home := t.TempDir()
	resolver := testResolver(home, "", "")
	managedFiles := []string{
		filepath.Join(home, ".local", "share", "alter", "bin", "mise"),
		filepath.Join(home, ".local", "state", "alter", "mise", "config.toml"),
		filepath.Join(home, ".local", "state", "alter", "mise", "data", "runtime"),
		filepath.Join(home, ".cache", "alter", "mise", "download"),
	}
	for _, path := range managedFiles {
		writeFile(t, path)
	}

	userFiles := []string{
		filepath.Join(home, ".tool-versions"),
		filepath.Join(home, ".config", "mise", "config.toml"),
		filepath.Join(home, ".asdf", "installs", "go", "1.25"),
		filepath.Join(home, ".local", "bin", "mise"),
	}
	for _, path := range userFiles {
		writeFile(t, path)
	}

	items, err := cleanupManagedMise(resolver)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("cleanup items = %d, want 5", len(items))
	}
	for _, path := range []string{
		filepath.Join(home, ".local", "share", "alter", "bin", "mise"),
		filepath.Join(home, ".local", "state", "alter", "mise"),
		filepath.Join(home, ".cache", "alter", "mise"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("managed path %q still exists or stat failed: %v", path, err)
		}
	}
	for _, path := range userFiles {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("user path %q was touched: %v", path, err)
		}
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
}
