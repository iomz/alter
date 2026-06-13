package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/iomz/alter/internal/plugin"
)

const miseBinaryName = "mise"

type MiseResolver interface {
	Resolve() (string, error)
	Candidates() ([]string, error)
}

type MisePathResolver struct {
	lookPath func(string) (string, error)
	homeDir  func() (string, error)
	stat     func(string) (os.FileInfo, error)
	abs      func(string) (string, error)
}

func NewMiseResolver() *MisePathResolver {
	return &MisePathResolver{
		lookPath: exec.LookPath,
		homeDir:  os.UserHomeDir,
		stat:     os.Stat,
		abs:      filepath.Abs,
	}
}

func (r *MisePathResolver) Resolve() (string, error) {
	candidates, err := r.Candidates()
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", NewMiseNotFoundError(r.searchedPaths())
	}
	return candidates[0], nil
}

func (r *MisePathResolver) Candidates() ([]string, error) {
	var candidates []string
	if path, err := r.lookPath(miseBinaryName); err == nil {
		absPath, err := r.abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve mise on PATH: %w", err)
		}
		candidates = append(candidates, absPath)
	}

	for _, path := range r.managedPaths() {
		info, err := r.stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect managed mise path %q: %w", path, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("managed mise path %q is a directory", path)
		}
		if info.Mode()&0o111 == 0 {
			return nil, fmt.Errorf("managed mise path %q is not executable", path)
		}
		absPath, err := r.abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve managed mise path %q: %w", path, err)
		}
		candidates = append(candidates, absPath)
	}

	return uniqueAbsolutePaths(candidates), nil
}

func (r *MisePathResolver) searchedPaths() []string {
	paths := []string{"PATH"}
	paths = append(paths, r.managedPaths()...)
	return paths
}

func (r *MisePathResolver) managedPaths() []string {
	home, err := r.homeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "share", "alter", "bin", "mise"),
		filepath.Join(home, ".local", "bin", "mise"),
	}
}

func uniqueAbsolutePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var unique []string
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			continue
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		unique = append(unique, clean)
	}
	return unique
}

type MiseNotFoundError struct {
	Paths []string
}

func NewMiseNotFoundError(paths []string) MiseNotFoundError {
	return MiseNotFoundError{Paths: paths}
}

func (e MiseNotFoundError) Error() string {
	return fmt.Sprintf(
		"mise not found; searched %s; run `alter setup mise` to inspect runtime setup",
		strings.Join(e.Paths, ", "),
	)
}

type MiseRunner struct {
	stdout   io.Writer
	stderr   io.Writer
	resolver MiseResolver
}

func NewMiseRunner(stdout, stderr io.Writer) *MiseRunner {
	return NewMiseRunnerWithResolver(stdout, stderr, NewMiseResolver())
}

func NewMiseRunnerWithResolver(stdout, stderr io.Writer, resolver MiseResolver) *MiseRunner {
	return &MiseRunner{stdout: stdout, stderr: stderr, resolver: resolver}
}

func (r *MiseRunner) Prepare(p plugin.Plugin) error {
	misePath, err := r.resolver.Resolve()
	if err != nil {
		return err
	}
	if hasMiseConfig(p.Path) {
		fmt.Fprintf(r.stderr, "warning: plugin %q has mise.toml; review and trust it before running untrusted code\n", p.Manifest.Plugin.Name)
	}
	cmd := exec.Command(misePath, "install")
	cmd.Dir = p.Path
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}

func (r *MiseRunner) Run(p plugin.Plugin, args ...string) ([]byte, error) {
	if p.Manifest.Runtime.Manager != "mise" {
		return nil, fmt.Errorf("unsupported runtime manager %q", p.Manifest.Runtime.Manager)
	}
	misePath, err := r.resolver.Resolve()
	if err != nil {
		return nil, err
	}

	entrypoint := p.Manifest.Plugin.Entrypoint
	if _, err := os.Stat(filepath.Join(p.Path, entrypoint)); err == nil {
		entrypoint = "./" + entrypoint
	}

	cmdArgs := append([]string{"exec", "--", entrypoint}, args...)
	cmd := exec.Command(misePath, cmdArgs...)
	cmd.Dir = p.Path
	cmd.Stderr = r.stderr
	return cmd.Output()
}

func hasMiseConfig(path string) bool {
	_, err := os.Stat(filepath.Join(path, "mise.toml"))
	return err == nil
}

var _ error = MiseNotFoundError{}
