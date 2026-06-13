package runtime

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iomz/alter/internal/plugin"
)

func TestMiseResolverReturnsPathCandidateFirst(t *testing.T) {
	home := t.TempDir()
	pathMise := filepath.Join(home, "bin", "mise")
	managedMise := filepath.Join(home, ".local", "share", "alter", "bin", "mise")

	resolver := testResolver(home, pathMise, managedMise)

	got, err := resolver.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got != pathMise {
		t.Fatalf("Resolve() = %q, want %q", got, pathMise)
	}
}

func TestMiseResolverReturnsManagedCandidateWhenPathMissing(t *testing.T) {
	home := t.TempDir()
	managedMise := filepath.Join(home, ".local", "share", "alter", "bin", "mise")

	resolver := testResolver(home, "", managedMise)

	got, err := resolver.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got != managedMise {
		t.Fatalf("Resolve() = %q, want %q", got, managedMise)
	}
}

func TestMiseResolverDeduplicatesCandidates(t *testing.T) {
	home := t.TempDir()
	managedMise := filepath.Join(home, ".local", "share", "alter", "bin", "mise")

	resolver := testResolver(home, managedMise, managedMise)

	got, err := resolver.Candidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != managedMise {
		t.Fatalf("Candidates() = %#v, want [%q]", got, managedMise)
	}
}

func TestMiseResolverReportsSearchedPaths(t *testing.T) {
	home := t.TempDir()
	resolver := testResolver(home, "", "")

	_, err := resolver.Resolve()
	if err == nil {
		t.Fatal("Resolve() error = nil, want mise not found error")
	}
	msg := err.Error()
	for _, want := range []string{
		"mise not found",
		"PATH",
		filepath.Join(home, ".local", "share", "alter", "bin", "mise"),
		filepath.Join(home, ".local", "bin", "mise"),
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
}

func TestMiseResolverRejectsManagedNonExecutable(t *testing.T) {
	home := t.TempDir()
	managedMise := filepath.Join(home, ".local", "share", "alter", "bin", "mise")
	resolver := testResolver(home, "", managedMise)
	resolver.stat = func(path string) (os.FileInfo, error) {
		if filepath.Clean(path) == managedMise {
			return testFileInfo{mode: 0o644}, nil
		}
		return nil, os.ErrNotExist
	}

	_, err := resolver.Resolve()
	if err == nil {
		t.Fatal("Resolve() error = nil, want non-executable error")
	}
	if !strings.Contains(err.Error(), "is not executable") {
		t.Fatalf("Resolve() error = %q, want non-executable error", err)
	}
}

func TestMiseRunnerSetsIsolatedMiseEnv(t *testing.T) {
	home := t.TempDir()
	for _, key := range []string{
		"MISE_SHELL",
		"MISE_ENV",
		"MISE_CONFIG_FILE",
		"MISE_DEFAULT_CONFIG_FILENAME",
		"ASDF_CONFIG_FILE",
		"ASDF_DATA_DIR",
	} {
		t.Setenv(key, "leak")
	}
	resolver := testResolver(home, "", "")
	runner := NewMiseRunnerWithResolver(io.Discard, io.Discard, resolver)

	miseEnv, err := runner.BuildMiseEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	env := envMap(miseEnv.Vars)
	want := map[string]string{
		"MISE_OVERRIDE_CONFIG_FILENAMES":        "alter.mise.toml",
		"MISE_OVERRIDE_TOOL_VERSIONS_FILENAME":  "alter.tool-versions",
		"MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES": "alter.tool-versions",
		"MISE_LEGACY_VERSION_FILE":              "false",
		"MISE_ASDF_COMPAT":                      "false",
		"MISE_GLOBAL_CONFIG_FILE":               filepath.Join(home, ".local", "state", "alter", "mise", "config.toml"),
		"MISE_DATA_DIR":                         filepath.Join(home, ".local", "state", "alter", "mise", "data"),
		"MISE_CACHE_DIR":                        filepath.Join(home, ".cache", "alter", "mise"),
		"MISE_STATE_DIR":                        filepath.Join(home, ".local", "state", "alter", "mise", "state"),
	}
	for key, value := range want {
		if env[key] != value {
			t.Fatalf("%s = %q, want %q", key, env[key], value)
		}
	}
	for _, key := range []string{
		"MISE_SHELL",
		"MISE_ENV",
		"MISE_CONFIG_FILE",
		"MISE_DEFAULT_CONFIG_FILENAME",
		"ASDF_CONFIG_FILE",
		"ASDF_DATA_DIR",
	} {
		if _, ok := env[key]; ok {
			t.Fatalf("%s leaked into mise env", key)
		}
	}
}

func TestDeclaredToolsReadsOnlyAlterMiseToml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tool-versions"), []byte("lua 5.4.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\nnode = \"24\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alter.mise.toml"), []byte("# no tools\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools, err := declaredTools(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 0 {
		t.Fatalf("declaredTools() = %#v, want no tools", tools)
	}
}

func TestPrepareSkipsMiseInstallWhenNoToolsDeclared(t *testing.T) {
	home := t.TempDir()
	pluginDir := filepath.Join(home, "plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "alter.mise.toml"), []byte("# no tools\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(home, "mise-called")
	fakeMise := filepath.Join(home, "mise")
	if err := os.WriteFile(fakeMise, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := testResolver(home, fakeMise, "")
	runner := NewMiseRunnerWithResolver(io.Discard, io.Discard, resolver)
	err := runner.Prepare(plugin.Plugin{
		Path: pluginDir,
		Manifest: plugin.Manifest{
			Plugin:  plugin.PluginSection{Name: "hello"},
			Runtime: plugin.RuntimeSection{Manager: "mise"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("mise install ran; marker stat error = %v", err)
	}
}

func TestRunUsesDirectModeWhenNoToolsDeclared(t *testing.T) {
	home := t.TempDir()
	pluginDir := filepath.Join(home, "plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, ".tool-versions"), []byte("lua 5.4.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "alter.mise.toml"), []byte("# no tools\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(pluginDir, "adapter")
	if err := os.WriteFile(entrypoint, []byte("#!/bin/sh\nprintf '{\"ok\":true,\"cwd\":\"%s\"}\\n' \"$PWD\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(home, "mise-called")
	fakeMise := filepath.Join(home, "mise")
	if err := os.WriteFile(fakeMise, []byte("#!/bin/sh\ntouch "+marker+"\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := testResolver(home, fakeMise, "")
	runner := NewMiseRunnerWithResolver(io.Discard, io.Discard, resolver)
	out, err := runner.Run(plugin.Plugin{
		Path: pluginDir,
		Manifest: plugin.Manifest{
			Plugin:  plugin.PluginSection{Name: "hello", Entrypoint: "adapter"},
			Runtime: plugin.RuntimeSection{Manager: "mise"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), pluginDir) {
		t.Fatalf("Run() output = %s, want cwd %s", out, pluginDir)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("mise exec ran; marker stat error = %v", err)
	}
}

func TestDeclaredToolsReadsAlterToolVersions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alter.tool-versions"), []byte("# plugin tools\nnode 24\npython 3.12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools, err := declaredTools(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(tools, ",") != "node@24,python@3.12" {
		t.Fatalf("declaredTools() = %#v, want node@24 and python@3.12", tools)
	}
}

func testResolver(home, pathMise, managedMise string) *MisePathResolver {
	files := map[string]struct{}{}
	if managedMise != "" {
		files[filepath.Clean(managedMise)] = struct{}{}
	}

	return &MisePathResolver{
		lookPath: func(string) (string, error) {
			if pathMise == "" {
				return "", errors.New("not found")
			}
			return pathMise, nil
		},
		homeDir: func() (string, error) {
			return home, nil
		},
		stat: func(path string) (os.FileInfo, error) {
			if _, ok := files[filepath.Clean(path)]; ok {
				return testFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		abs: filepath.Abs,
	}
}

func envMap(env []string) map[string]string {
	result := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

type testFileInfo struct {
	mode os.FileMode
}

func (testFileInfo) Name() string { return "mise" }
func (testFileInfo) Size() int64  { return 1 }
func (i testFileInfo) Mode() os.FileMode {
	if i.mode == 0 {
		return 0o755
	}
	return i.mode
}
func (testFileInfo) ModTime() time.Time { return time.Time{} }
func (testFileInfo) IsDir() bool        { return false }
func (testFileInfo) Sys() any           { return nil }
