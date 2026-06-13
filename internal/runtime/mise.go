package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iomz/alter/internal/plugin"
	"github.com/pelletier/go-toml/v2"
)

const miseBinaryName = "mise"
const miseConfigFileName = "alter.mise.toml"

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
	path, err := r.ManagedInstallPath()
	if err != nil {
		return nil
	}
	localBinPath, err := r.LocalBinPath()
	if err != nil {
		return []string{path}
	}
	return []string{path, localBinPath}
}

func (r *MisePathResolver) managedPath(parts ...string) (string, error) {
	home, err := r.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("resolve home directory: empty path")
	}
	return r.abs(filepath.Join(append([]string{home}, parts...)...))
}

func (r *MisePathResolver) ManagedInstallPath() (string, error) {
	return r.managedPath(".local", "share", "alter", "bin", "mise")
}

func (r *MisePathResolver) LocalBinPath() (string, error) {
	return r.managedPath(".local", "bin", "mise")
}

func (r *MisePathResolver) ManagedStateDir() (string, error) {
	return r.managedPath(".local", "state", "alter", "mise")
}

func (r *MisePathResolver) ManagedDataDir() (string, error) {
	return r.managedPath(".local", "state", "alter", "mise", "data")
}

func (r *MisePathResolver) ManagedMiseStateDir() (string, error) {
	return r.managedPath(".local", "state", "alter", "mise", "state")
}

func (r *MisePathResolver) ManagedCacheDir() (string, error) {
	return r.managedPath(".cache", "alter", "mise")
}

func (r *MisePathResolver) ManagedConfigFile() (string, error) {
	return r.managedPath(".local", "state", "alter", "mise", "config.toml")
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

type MiseEnvironment struct {
	Vars             []string          `json:"-"`
	Values           map[string]string `json:"values"`
	GlobalConfigFile string            `json:"globalConfigFile"`
	DataDir          string            `json:"dataDir"`
	CacheDir         string            `json:"cacheDir"`
	StateDir         string            `json:"stateDir"`
}

type RuntimeConfig struct {
	Path  string   `json:"path"`
	Tools []string `json:"tools"`
}

type DiagnosticReport struct {
	PluginWorkspace string            `json:"pluginWorkspace"`
	MiseBinary      string            `json:"miseBinary"`
	Environment     map[string]string `json:"environment"`
	RuntimeConfig   RuntimeConfig     `json:"runtimeConfig"`
	InstallSkipped  bool              `json:"installSkipped"`
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
		fmt.Fprintf(r.stderr, "warning: plugin %q has %s; review and trust it before running untrusted code\n", p.Manifest.Plugin.Name, miseConfigFileName)
	}
	tools, err := declaredTools(p.Path)
	if err != nil {
		return err
	}
	if len(tools) == 0 {
		return nil
	}
	cmd := exec.Command(misePath, "install")
	cmd.Dir = p.Path
	if err := r.configureMiseCommand(cmd); err != nil {
		return err
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mise install failed: %w\n%s", err, strings.TrimSpace(output.String()))
	}
	return nil
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
	if err := r.configureMiseCommand(cmd); err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mise exec failed: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func hasMiseConfig(path string) bool {
	_, err := os.Stat(filepath.Join(path, miseConfigFileName))
	return err == nil
}

func (r *MiseRunner) configureMiseCommand(cmd *exec.Cmd) error {
	env, err := r.BuildMiseEnvironment()
	if err != nil {
		return err
	}
	cmd.Env = env.Vars
	return nil
}

func (r *MiseRunner) BuildMiseEnvironment() (MiseEnvironment, error) {
	resolver, ok := r.resolver.(*MisePathResolver)
	if !ok {
		values := safeBaseEnv()
		values["MISE_OVERRIDE_CONFIG_FILENAMES"] = miseConfigFileName
		return MiseEnvironment{Vars: flattenEnv(values), Values: values}, nil
	}
	stateDir, err := resolver.ManagedStateDir()
	if err != nil {
		return MiseEnvironment{}, err
	}
	dataDir, err := resolver.ManagedDataDir()
	if err != nil {
		return MiseEnvironment{}, err
	}
	cacheDir, err := resolver.ManagedCacheDir()
	if err != nil {
		return MiseEnvironment{}, err
	}
	configFile, err := resolver.ManagedConfigFile()
	if err != nil {
		return MiseEnvironment{}, err
	}
	miseStateDir, err := resolver.ManagedMiseStateDir()
	if err != nil {
		return MiseEnvironment{}, err
	}
	for _, dir := range []string{stateDir, dataDir, cacheDir, miseStateDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return MiseEnvironment{}, fmt.Errorf("create isolated mise directory %q: %w", dir, err)
		}
	}
	values := safeBaseEnv()
	values["MISE_OVERRIDE_CONFIG_FILENAMES"] = miseConfigFileName
	values["MISE_GLOBAL_CONFIG_FILE"] = configFile
	values["MISE_DATA_DIR"] = dataDir
	values["MISE_CACHE_DIR"] = cacheDir
	values["MISE_STATE_DIR"] = miseStateDir
	return MiseEnvironment{
		Vars:             flattenEnv(values),
		Values:           values,
		GlobalConfigFile: configFile,
		DataDir:          dataDir,
		CacheDir:         cacheDir,
		StateDir:         miseStateDir,
	}, nil
}

func (r *MiseRunner) Diagnostics(p plugin.Plugin) (DiagnosticReport, error) {
	misePath, err := r.resolver.Resolve()
	if err != nil {
		return DiagnosticReport{}, err
	}
	env, err := r.BuildMiseEnvironment()
	if err != nil {
		return DiagnosticReport{}, err
	}
	tools, err := declaredTools(p.Path)
	if err != nil {
		return DiagnosticReport{}, err
	}
	configPath := filepath.Join(p.Path, miseConfigFileName)
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		configPath = ""
	} else if err != nil {
		return DiagnosticReport{}, fmt.Errorf("inspect plugin runtime config: %w", err)
	}
	return DiagnosticReport{
		PluginWorkspace: p.Path,
		MiseBinary:      misePath,
		Environment:     env.Values,
		RuntimeConfig: RuntimeConfig{
			Path:  configPath,
			Tools: tools,
		},
		InstallSkipped: len(tools) == 0,
	}, nil
}

func safeBaseEnv() map[string]string {
	values := make(map[string]string)
	for _, key := range []string{"HOME", "PATH", "TMPDIR", "TERM", "LANG", "LC_ALL"} {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			values[key] = value
		}
	}
	return values
}

func flattenEnv(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

func declaredTools(pluginPath string) ([]string, error) {
	path := filepath.Join(pluginPath, miseConfigFileName)
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugin runtime config: %w", err)
	}
	var config struct {
		Tools map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("parse plugin runtime config: %w", err)
	}
	tools := make([]string, 0, len(config.Tools))
	for name := range config.Tools {
		tools = append(tools, name)
	}
	sort.Strings(tools)
	return tools, nil
}

var _ error = MiseNotFoundError{}
