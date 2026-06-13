package runtime

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMiseBootstrapperInstallsToManagedPath(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "alter", "bin", "mise")
	var gotURL string
	var gotScriptPath string
	var gotTarget string

	bootstrapper := &MiseBootstrapper{
		installPath: func() (string, error) {
			return target, nil
		},
		download: func(_ context.Context, url string) ([]byte, error) {
			gotURL = url
			return []byte("#!/bin/sh\n"), nil
		},
		runScript: func(_ context.Context, scriptPath, targetPath string, _, _ io.Writer) error {
			gotScriptPath = scriptPath
			gotTarget = targetPath
			if err := os.WriteFile(targetPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
				return err
			}
			return nil
		},
		mkdirAll:  os.MkdirAll,
		writeFile: os.WriteFile,
		remove:    os.Remove,
		tempDir:   tempDir,
		stdout:    io.Discard,
		stderr:    io.Discard,
	}

	got, err := bootstrapper.Install(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("Install() = %q, want %q", got, target)
	}
	if gotURL != miseInstallURL {
		t.Fatalf("download URL = %q, want %q", gotURL, miseInstallURL)
	}
	if gotTarget != target {
		t.Fatalf("run target = %q, want %q", gotTarget, target)
	}
	if gotScriptPath == "" {
		t.Fatal("script path was empty")
	}
	if _, err := os.Stat(gotScriptPath); !os.IsNotExist(err) {
		t.Fatalf("installer script still exists, stat error = %v", err)
	}
}

func TestManagedInstallPathUsesAlterStorage(t *testing.T) {
	home := t.TempDir()
	resolver := testResolver(home, "", "")

	got, err := resolver.ManagedInstallPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "share", "alter", "bin", "mise")
	if got != want {
		t.Fatalf("ManagedInstallPath() = %q, want %q", got, want)
	}
}
