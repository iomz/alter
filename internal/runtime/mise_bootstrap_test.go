package runtime

import (
	"bytes"
	"context"
	"errors"
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
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	bootstrapper := &MiseBootstrapper{
		installPath: func() (string, error) {
			return target, nil
		},
		download: func(_ context.Context, url string) ([]byte, error) {
			gotURL = url
			return []byte("#!/bin/sh\n"), nil
		},
		runScript: func(_ context.Context, scriptPath, targetPath string) ([]byte, error) {
			gotScriptPath = scriptPath
			gotTarget = targetPath
			info, err := os.Stat(scriptPath)
			if err != nil {
				return nil, err
			}
			if info.Mode().Perm() != 0o600 {
				return nil, errors.New("installer script permissions are not 0600")
			}
			if err := os.WriteFile(targetPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
				return nil, err
			}
			return []byte("installer success output\n"), nil
		},
		mkdirAll:   os.MkdirAll,
		createTemp: os.CreateTemp,
		remove:     os.Remove,
		tempDir:    tempDir,
		stdout:     &stdout,
		stderr:     &stderr,
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
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no installer output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no installer output", stderr.String())
	}
}

func TestMiseBootstrapperShowsInstallerOutputOnlyOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "alter", "bin", "mise")
	var stderr bytes.Buffer

	bootstrapper := &MiseBootstrapper{
		installPath: func() (string, error) {
			return target, nil
		},
		download: func(context.Context, string) ([]byte, error) {
			return []byte("#!/bin/sh\n"), nil
		},
		runScript: func(context.Context, string, string) ([]byte, error) {
			return []byte("raw installer failure\n"), errors.New("installer failed")
		},
		mkdirAll:   os.MkdirAll,
		createTemp: os.CreateTemp,
		remove:     os.Remove,
		tempDir:    tempDir,
		stdout:     io.Discard,
		stderr:     &stderr,
	}

	_, err := bootstrapper.Install(context.Background())
	if err == nil {
		t.Fatal("Install() error = nil, want installer error")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("raw installer failure")) {
		t.Fatalf("stderr = %q, want raw installer output", stderr.String())
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
