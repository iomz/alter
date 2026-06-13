package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
