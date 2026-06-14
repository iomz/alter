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
const miseToolVersionsFileName = "alter.tool-versions"

type RuntimeMode string

const (
	RuntimeModeDirect RuntimeMode = "direct"
	RuntimeModeMise   RuntimeMode = "mise"
)

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
	Path             string   `json:"path"`
	ToolVersionsPath string   `json:"toolVersionsPath"`
	Tools            []string `json:"tools"`
}

type DiagnosticReport struct {
	PluginName         string            `json:"pluginName"`
	PluginWorkspace    string            `json:"pluginWorkspace"`
	AdapterEntrypoint  string            `json:"adapterEntrypoint"`
	RuntimeMode        RuntimeMode       `json:"runtimeMode"`
	MiseBinary         string            `json:"miseBinary"`
	MiseCWD            string            `json:"miseCwd"`
	Environment        map[string]string `json:"environment"`
	RuntimeConfig      RuntimeConfig     `json:"runtimeConfig"`
	InstallSkipped     bool              `json:"installSkipped"`
	MiseConfigExists   bool              `json:"miseConfigExists"`
	ToolVersionsExists bool              `json:"toolVersionsExists"`
}

func NewMiseRunner(stdout, stderr io.Writer) *MiseRunner {
	return NewMiseRunnerWithResolver(stdout, stderr, NewMiseResolver())
}

func NewMiseRunnerWithResolver(stdout, stderr io.Writer, resolver MiseResolver) *MiseRunner {
	return &MiseRunner{stdout: stdout, stderr: stderr, resolver: resolver}
}

func (r *MiseRunner) Prepare(p plugin.Plugin) error {
	decision, err := r.Decide(p)
	if err != nil {
		return err
	}
	r.logDecision(p, decision)
	if decision.Mode == RuntimeModeDirect {
		return nil
	}
	if decision.MiseConfigExists {
		r.printRuntimeConfigNotice(p, decision)
	}
	if len(decision.DeclaredTools) == 0 {
		return nil
	}
	cmd := exec.Command(decision.MiseBinary, "install")
	cmd.Dir = p.Path
	if err := r.configureMiseCommand(cmd); err != nil {
		return err
	}
	r.debugCommand("mise install", cmd)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mise install failed: %w\n%s", err, strings.TrimSpace(output.String()))
	}
	return nil
}

func (r *MiseRunner) printRuntimeConfigNotice(p plugin.Plugin, decision RuntimeDecision) {
	fmt.Fprintf(r.stderr, "warning: plugin %q declares mise-managed runtime config\n", p.Manifest.Plugin.Name)
	fmt.Fprintf(r.stderr, "  config: %s\n", decision.MiseConfigPath)
	fmt.Fprintf(r.stderr, "  declared tools: %s\n", formatTools(decision.DeclaredTools))
	fmt.Fprintf(r.stderr, "  what it means: alter will let mise install or reuse these tool versions, then run the plugin adapter from this workspace.\n")
	fmt.Fprintf(r.stderr, "  what you are trusting: this local plugin directory, its %s, and its adapter entrypoint %q.\n", miseConfigFileName, p.Manifest.Plugin.Entrypoint)
	fmt.Fprintf(r.stderr, "  running code: mise may download tool archives; the adapter process may execute local commands with your user permissions.\n")
	fmt.Fprintf(r.stderr, "  how to trust: inspect %s, %s, and %s; confirm declared tools and adapter code match your expectation; then run the command again. If not trusted, do not run this plugin.\n",
		filepath.Join(p.Path, plugin.ManifestFileName),
		decision.MiseConfigPath,
		filepath.Join(p.Path, p.Manifest.Plugin.Entrypoint),
	)
	fmt.Fprintf(r.stderr, "  note: current prototype has no persistent trust store; this notice is informational.\n")
}

func (r *MiseRunner) Run(p plugin.Plugin, args ...string) ([]byte, error) {
	if p.Manifest.Runtime.Manager != "mise" {
		return nil, fmt.Errorf("unsupported runtime manager %q", p.Manifest.Runtime.Manager)
	}
	decision, err := r.Decide(p)
	if err != nil {
		return nil, err
	}
	r.logDecision(p, decision)

	entrypoint := p.Manifest.Plugin.Entrypoint
	if _, err := os.Stat(filepath.Join(p.Path, entrypoint)); err == nil {
		entrypoint = "./" + entrypoint
	}

	if decision.Mode == RuntimeModeDirect {
		cmd := exec.Command(entrypoint, args...)
		cmd.Dir = p.Path
		cmd.Env = flattenEnv(safeBaseEnv())
		r.debugCommand("direct adapter", cmd)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("adapter execution failed: %w\n%s", err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}

	cmdArgs := append([]string{"exec", "--", entrypoint}, args...)
	cmd := exec.Command(decision.MiseBinary, cmdArgs...)
	cmd.Dir = p.Path
	if err := r.configureMiseCommand(cmd); err != nil {
		return nil, err
	}
	r.debugCommand("mise exec", cmd)
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

func hasToolVersionsConfig(path string) bool {
	_, err := os.Stat(filepath.Join(path, miseToolVersionsFileName))
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
		values["MISE_OVERRIDE_TOOL_VERSIONS_FILENAME"] = miseToolVersionsFileName
		values["MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES"] = miseToolVersionsFileName
		values["MISE_LEGACY_VERSION_FILE"] = "false"
		values["MISE_ASDF_COMPAT"] = "false"
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
	values["MISE_OVERRIDE_TOOL_VERSIONS_FILENAME"] = miseToolVersionsFileName
	values["MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES"] = miseToolVersionsFileName
	values["MISE_LEGACY_VERSION_FILE"] = "false"
	values["MISE_ASDF_COMPAT"] = "false"
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

type RuntimeDecision struct {
	Mode               RuntimeMode
	DeclaredTools      []string
	MiseBinary         string
	MiseConfigPath     string
	ToolVersionsPath   string
	MiseConfigExists   bool
	ToolVersionsExists bool
	InstallSkipped     bool
}

func (r *MiseRunner) Decide(p plugin.Plugin) (RuntimeDecision, error) {
	tools, err := declaredTools(p.Path)
	if err != nil {
		return RuntimeDecision{}, err
	}
	miseConfigPath := filepath.Join(p.Path, miseConfigFileName)
	toolVersionsPath := filepath.Join(p.Path, miseToolVersionsFileName)
	decision := RuntimeDecision{
		Mode:               RuntimeModeDirect,
		DeclaredTools:      tools,
		MiseConfigPath:     miseConfigPath,
		ToolVersionsPath:   toolVersionsPath,
		MiseConfigExists:   hasMiseConfig(p.Path),
		ToolVersionsExists: hasToolVersionsConfig(p.Path),
		InstallSkipped:     true,
	}
	if len(tools) == 0 {
		return decision, nil
	}
	misePath, err := r.resolver.Resolve()
	if err != nil {
		return RuntimeDecision{}, err
	}
	decision.Mode = RuntimeModeMise
	decision.MiseBinary = misePath
	decision.InstallSkipped = false
	return decision, nil
}

func (r *MiseRunner) Diagnostics(p plugin.Plugin) (DiagnosticReport, error) {
	decision, err := r.Decide(p)
	if err != nil {
		return DiagnosticReport{}, err
	}
	envValues := map[string]string{}
	if decision.Mode == RuntimeModeMise {
		env, err := r.BuildMiseEnvironment()
		if err != nil {
			return DiagnosticReport{}, err
		}
		envValues = env.Values
	} else {
		envValues = safeBaseEnv()
	}
	configPath := ""
	if decision.MiseConfigExists {
		configPath = decision.MiseConfigPath
	}
	toolVersionsPath := ""
	if decision.ToolVersionsExists {
		toolVersionsPath = decision.ToolVersionsPath
	}
	return DiagnosticReport{
		PluginName:        p.Manifest.Plugin.Name,
		PluginWorkspace:   p.Path,
		AdapterEntrypoint: p.Manifest.Plugin.Entrypoint,
		RuntimeMode:       decision.Mode,
		MiseBinary:        decision.MiseBinary,
		MiseCWD:           p.Path,
		Environment:       envValues,
		RuntimeConfig: RuntimeConfig{
			Path:             configPath,
			ToolVersionsPath: toolVersionsPath,
			Tools:            decision.DeclaredTools,
		},
		InstallSkipped:     decision.InstallSkipped,
		MiseConfigExists:   decision.MiseConfigExists,
		ToolVersionsExists: decision.ToolVersionsExists,
	}, nil
}

func (r *MiseRunner) logDecision(p plugin.Plugin, decision RuntimeDecision) {
	if os.Getenv("ALTER_LOG") != "debug" {
		return
	}
	envValues := safeBaseEnv()
	if decision.Mode == RuntimeModeMise {
		if env, err := r.BuildMiseEnvironment(); err == nil {
			envValues = env.Values
		}
	}
	fmt.Fprintf(r.stderr, "alter debug: plugin=%s\n", p.Manifest.Plugin.Name)
	fmt.Fprintf(r.stderr, "alter debug: plugin_workspace=%s\n", p.Path)
	fmt.Fprintf(r.stderr, "alter debug: adapter_entrypoint=%s\n", p.Manifest.Plugin.Entrypoint)
	fmt.Fprintf(r.stderr, "alter debug: runtime_mode=%s\n", decision.Mode)
	fmt.Fprintf(r.stderr, "alter debug: %s exists=%t\n", miseConfigFileName, decision.MiseConfigExists)
	fmt.Fprintf(r.stderr, "alter debug: %s exists=%t\n", miseToolVersionsFileName, decision.ToolVersionsExists)
	fmt.Fprintf(r.stderr, "alter debug: declared_runtime_tools=%s\n", formatTools(decision.DeclaredTools))
	fmt.Fprintf(r.stderr, "alter debug: mise_install_skipped=%t\n", decision.InstallSkipped)
	fmt.Fprintf(r.stderr, "alter debug: mise_binary=%s\n", valueOrNone(decision.MiseBinary))
	fmt.Fprintf(r.stderr, "alter debug: mise_cwd=%s\n", p.Path)
	for _, key := range sortedKeys(envValues) {
		if strings.HasPrefix(key, "MISE_") || strings.HasPrefix(key, "ASDF_") {
			fmt.Fprintf(r.stderr, "alter debug: env %s=%s\n", key, envValues[key])
		}
	}
}

func (r *MiseRunner) debugCommand(label string, cmd *exec.Cmd) {
	if os.Getenv("ALTER_LOG") != "debug" {
		return
	}
	fmt.Fprintf(r.stderr, "alter debug: command[%s]=%s\n", label, shellQuoteCommand(cmd.Args))
	fmt.Fprintf(r.stderr, "alter debug: command[%s].cwd=%s\n", label, cmd.Dir)
}

func formatTools(tools []string) string {
	if len(tools) == 0 {
		return "none"
	}
	return strings.Join(tools, ",")
}

func valueOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func shellQuoteCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n'\"\\$") {
			quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
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
	names := make(map[string]struct{})
	path := filepath.Join(pluginPath, miseConfigFileName)
	body, err := os.ReadFile(path)
	if err == nil {
		var config struct {
			Tools map[string]any `toml:"tools"`
		}
		if err := toml.Unmarshal(body, &config); err != nil {
			return nil, fmt.Errorf("parse plugin runtime config: %w", err)
		}
		for name, version := range config.Tools {
			names[toolLabel(name, version)] = struct{}{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read plugin runtime config: %w", err)
	}

	toolVersionsPath := filepath.Join(pluginPath, miseToolVersionsFileName)
	toolVersionsBody, err := os.ReadFile(toolVersionsPath)
	if err == nil {
		for _, line := range strings.Split(string(toolVersionsBody), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				label := fields[0]
				if len(fields) > 1 {
					label = fields[0] + "@" + fields[1]
				}
				names[label] = struct{}{}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read plugin runtime tool versions: %w", err)
	}

	tools := make([]string, 0, len(names))
	for name := range names {
		tools = append(tools, name)
	}
	sort.Strings(tools)
	return tools, nil
}

func toolLabel(name string, value any) string {
	switch v := value.(type) {
	case string:
		if v != "" {
			return name + "@" + v
		}
	case fmt.Stringer:
		if v.String() != "" {
			return name + "@" + v.String()
		}
	}
	return name
}

var _ error = MiseNotFoundError{}
