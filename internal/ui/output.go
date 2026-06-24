package ui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/iomz/alter/internal/plugin"
	"github.com/iomz/alter/internal/runtime"
	"github.com/iomz/alter/internal/trust"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36"))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	okStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	infoStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	warnStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	errStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))

	promptTheme = huh.ThemeBase()
)

func PrintRuntimeFound(out io.Writer, path string) {
	printHeading(out, "mise runtime")
	printRows(out, [][2]string{
		{"status", okStyle.Render("found")},
		{"path", path},
	})
}

func PrintRuntimeMissing(out io.Writer, err error) {
	printHeading(out, "mise runtime")
	printRows(out, [][2]string{
		{"status", errStyle.Render("missing")},
		{"detail", err.Error()},
	})
}

func PrintMiseBootstrapExplanation(out io.Writer, installPath string) error {
	body := fmt.Sprintf(`
mise is required for alter-managed plugin runtimes.

What alter will do:

- download the official mise installer from https://mise.run
- run it with MISE_INSTALL_PATH=%s
- keep the binary under alter-managed storage
- use the full mise path internally

What alter will NOT do:

- modify shell startup files
- run sudo
- install without confirmation
- configure future shell activation
`, installPath)
	rendered, err := renderMarkdown("## setup mise\n" + body)
	if err != nil {
		if _, writeErr := fmt.Fprintln(out, titleStyle.Render("setup mise")); writeErr != nil {
			return writeErr
		}
		if _, writeErr := fmt.Fprintln(out, strings.TrimSpace(body)); writeErr != nil {
			return writeErr
		}
		return err
	}
	_, err = fmt.Fprint(out, rendered)
	return err
}

func ConfirmMiseBootstrap(out io.Writer, in io.Reader) (bool, error) {
	confirmed := false
	confirm := huh.NewConfirm().
		Title("Install mise into alter-managed storage?").
		Affirmative("Install").
		Negative("Cancel").
		Value(&confirmed)
	confirm.WithTheme(promptTheme)
	err := confirm.RunAccessible(out, in)
	return confirmed, err
}

func ConfirmPluginTrust(out io.Writer, in io.Reader, name string) (bool, error) {
	confirmed := false
	confirm := huh.NewConfirm().
		Title(fmt.Sprintf("Trust plugin %q?", name)).
		Description("Trust stores current file fingerprints and allows mise-managed runtime execution until a fingerprint changes.").
		Affirmative("Trust").
		Negative("Abort").
		Value(&confirmed)
	confirm.WithTheme(promptTheme)
	err := confirm.RunAccessible(out, in)
	return confirmed, err
}

func PrintStub(out io.Writer, title, body string) error {
	rendered, err := renderMarkdown("## " + title + "\n\n" + body)
	if err != nil {
		if _, writeErr := fmt.Fprintln(out, titleStyle.Render(title)); writeErr != nil {
			return writeErr
		}
		if _, writeErr := fmt.Fprintln(out, strings.TrimSpace(body)); writeErr != nil {
			return writeErr
		}
		return err
	}
	_, err = fmt.Fprint(out, rendered)
	return err
}

func PrintPromptDeferred(out io.Writer) {
	fmt.Fprintln(out, promptTheme.Focused.Description.Render("interactive prompts: planned"))
}

func PrintRuntimeInstalled(out io.Writer, path string) {
	printHeading(out, "mise runtime")
	printRows(out, [][2]string{
		{"status", okStyle.Render("installed")},
		{"path", path},
	})
}

func PrintCleanupReport(out io.Writer, items []runtime.CleanupItem) {
	printHeading(out, "setup cleanup")
	for _, item := range items {
		status := "kept"
		style := warnStyle
		if item.Removed {
			status = "removed"
			style = okStyle
		}
		fmt.Fprintf(out, "%s %s\n", style.Width(9).Render(status), item.Label)
		fmt.Fprintf(out, "          %s\n", mutedStyle.Render(item.Path))
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s %s\n", infoStyle.Render("safety"), "user shell configs and global mise/asdf files were not touched")
}

func PrintPluginList(out io.Writer, plugins []plugin.Plugin) {
	printHeading(out, "plugins")
	if len(plugins) == 0 {
		fmt.Fprintln(out, mutedStyle.Render("none found"))
		return
	}
	fmt.Fprintf(out, "%s  %s  %s\n",
		labelStyle.Width(16).Render("name"),
		labelStyle.Width(8).Render("mcp"),
		labelStyle.Render("description"),
	)
	fmt.Fprintf(out, "%s  %s  %s\n",
		mutedStyle.Width(16).Render(strings.Repeat("-", 16)),
		mutedStyle.Width(8).Render(strings.Repeat("-", 8)),
		mutedStyle.Render(strings.Repeat("-", 32)),
	)
	for _, p := range plugins {
		mcpState := "off"
		if p.Manifest.MCP.Enabled {
			mcpState = "on"
		}
		fmt.Fprintf(out, "%s  %s  %s\n",
			valueStyle.Width(16).Render(p.Manifest.Plugin.Name),
			valueStyle.Width(8).Render(mcpState),
			p.Manifest.Plugin.Description,
		)
	}
}

func PrintPluginManifest(out io.Writer, p plugin.Plugin) {
	printHeading(out, "plugin inspect")
	printSection(out, "plugin")
	printRows(out, [][2]string{
		{"name", p.Manifest.Plugin.Name},
		{"description", p.Manifest.Plugin.Description},
		{"maintainer", emptyValue(p.Manifest.Plugin.Maintainer)},
		{"entrypoint", p.Manifest.Plugin.Entrypoint},
		{"path", p.Path},
	})
	printSection(out, "upstream")
	printRows(out, [][2]string{
		{"name", emptyValue(p.Manifest.Upstream.Name)},
		{"repository", emptyValue(p.Manifest.Upstream.Repository)},
	})
	printSection(out, "runtime")
	printRows(out, [][2]string{
		{"manager", p.Manifest.Runtime.Manager},
	})
	printSection(out, "mcp")
	enabled := "disabled"
	if p.Manifest.MCP.Enabled {
		enabled = okStyle.Render("enabled")
	}
	printRows(out, [][2]string{
		{"status", enabled},
		{"namespace", emptyValue(p.Manifest.MCP.Namespace)},
	})
}

func PrintPluginDoctorReport(out io.Writer, report plugin.DoctorReport) {
	printHeading(out, "plugin doctor")
	status := okStyle.Render(report.Status)
	if len(report.Warnings) > 0 {
		status = warnStyle.Render("warning")
	}
	printRows(out, [][2]string{
		{"name", report.Name},
		{"status", status},
		{"path", report.Path},
		{"manifest", report.Manifest},
		{"entrypoint", emptyValue(report.Entrypoint)},
	})
	if len(report.Warnings) > 0 {
		printSection(out, "warnings")
		for _, warning := range report.Warnings {
			fmt.Fprintf(out, "%s %s\n", warnStyle.Render("warning"), warning)
		}
	}
}

func PrintRuntimeDiagnostics(out io.Writer, report runtime.DiagnosticReport) {
	printHeading(out, "runtime diagnostics")
	printSection(out, "plugin")
	printRows(out, [][2]string{
		{"name", report.PluginName},
		{"workspace", report.PluginWorkspace},
		{"entrypoint", report.AdapterEntrypoint},
	})

	printSection(out, "runtime")
	mode := string(report.RuntimeMode)
	if report.RuntimeMode == runtime.RuntimeModeDirect {
		mode = okStyle.Render(mode)
	} else {
		mode = infoStyle.Render(mode)
	}
	install := warnStyle.Render("required")
	if report.InstallSkipped {
		install = okStyle.Render("skipped")
	}
	printRows(out, [][2]string{
		{"mode", mode},
		{"mise install", install},
		{"declared tools", toolsValue(report.RuntimeConfig.Tools)},
	})

	printSection(out, "trust")
	PrintTrustRows(out, report.Trust)

	printSection(out, "config")
	printRows(out, [][2]string{
		{"alter.mise.toml", configValue(report.MiseConfigExists, report.RuntimeConfig.Path)},
		{"alter.tool-versions", configValue(report.ToolVersionsExists, report.RuntimeConfig.ToolVersionsPath)},
	})

	printSection(out, "mise isolation")
	if report.MiseBinary == "" {
		printRows(out, [][2]string{
			{"mise binary", mutedStyle.Render("not used")},
		})
		return
	}
	printRows(out, [][2]string{
		{"mise binary", report.MiseBinary},
		{"mise cwd", report.MiseCWD},
		{"user/global config", okStyle.Render("ignored")},
	})
	printSection(out, "mise env")
	var envRows [][2]string
	for _, key := range sortedKeys(report.Environment) {
		if strings.HasPrefix(key, "MISE_") || strings.HasPrefix(key, "ASDF_") {
			envRows = append(envRows, [2]string{key, report.Environment[key]})
		}
	}
	printRows(out, envRows)
}

func PrintTrustReview(out io.Writer, p plugin.Plugin, report runtime.DiagnosticReport) {
	printHeading(out, "plugin trust")
	printSection(out, "plugin")
	printRows(out, [][2]string{
		{"name", p.Manifest.Plugin.Name},
		{"workspace", p.Path},
		{"entrypoint", p.Manifest.Plugin.Entrypoint},
	})
	printSection(out, "runtime")
	printRows(out, [][2]string{
		{"mode", string(report.RuntimeMode)},
		{"declared tools", toolsValue(report.RuntimeConfig.Tools)},
		{"alter.mise.toml", configValue(report.MiseConfigExists, report.RuntimeConfig.Path)},
		{"alter.tool-versions", configValue(report.ToolVersionsExists, report.RuntimeConfig.ToolVersionsPath)},
	})
	printSection(out, "review")
	fmt.Fprintln(out, "Trust allows this plugin to run mise-managed runtime setup and adapter code until one fingerprint changes.")
	fmt.Fprintln(out, "Review files below before confirming.")
	printSection(out, "fingerprints")
	PrintTrustRows(out, report.Trust)
	printSection(out, "meaning")
	fmt.Fprintln(out, "alter.mise.toml is plugin-owned runtime policy. It declares tools mise may install or reuse.")
	fmt.Fprintln(out, "Trusting means accepting this local plugin directory, runtime config, and adapter entrypoint as code you are willing to run.")
	fmt.Fprintln(out, "Running code means mise may download tool archives and the adapter may execute local commands with your user permissions.")
}

func PrintTrustRows(out io.Writer, eval trust.Evaluation) {
	status := string(eval.Status)
	switch eval.Status {
	case trust.StatusTrusted, trust.StatusNotRequired:
		status = okStyle.Render(status)
	case trust.StatusInvalidated:
		status = warnStyle.Render(status)
	default:
		status = errStyle.Render(status)
	}
	printRows(out, [][2]string{
		{"status", status},
		{"reason", emptyValue(eval.Reason)},
		{"store", emptyValue(eval.StorePath)},
	})
	if eval.Stored != nil {
		printRows(out, [][2]string{
			{"trusted at", emptyValue(eval.Stored.TrustedAt)},
		})
	}
	printRows(out, [][2]string{
		{"workspace", eval.Current.WorkspacePath},
		{"manifest", hashValue(eval.Current.ManifestPath, eval.Current.ManifestHash)},
		{"alter.mise.toml", hashValue(eval.Current.MisePath, eval.Current.MiseHash)},
		{"alter.tool-versions", hashValue(eval.Current.ToolVersionsPath, eval.Current.ToolVersionsHash)},
		{"entrypoint", hashValue(eval.Current.EntrypointPath, eval.Current.EntrypointHash)},
	})
	if len(eval.Mismatches) > 0 {
		printSection(out, "invalidated")
		for _, mismatch := range eval.Mismatches {
			fmt.Fprintf(out, "%s %s\n", warnStyle.Render("changed"), mismatch)
		}
	}
}

func PrintTrustSaved(out io.Writer, record trust.Record, storePath string) {
	printHeading(out, "plugin trust")
	printRows(out, [][2]string{
		{"status", okStyle.Render("trusted")},
		{"plugin", record.Name},
		{"store", storePath},
		{"trusted at", record.TrustedAt},
	})
}

func PrintTrustRemoved(out io.Writer, name, storePath string, removed bool) {
	printHeading(out, "plugin trust")
	status := warnStyle.Render("not found")
	if removed {
		status = okStyle.Render("removed")
	}
	printRows(out, [][2]string{
		{"status", status},
		{"plugin", name},
		{"store", storePath},
	})
}

func PrintTrustNotRequired(out io.Writer, p plugin.Plugin) {
	printHeading(out, "plugin trust")
	printRows(out, [][2]string{
		{"status", okStyle.Render("not required")},
		{"plugin", p.Manifest.Plugin.Name},
		{"reason", "direct runtime has no declared tools"},
	})
}

func Warning(s string) string {
	return warnStyle.Render(s)
}

func Error(s string) string {
	return errStyle.Render(s)
}

func printHeading(out io.Writer, title string) {
	fmt.Fprintln(out, titleStyle.Render(title))
}

func printSection(out io.Writer, title string) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, sectionStyle.Render(title))
}

func printRows(out io.Writer, rows [][2]string) {
	width := 0
	for _, row := range rows {
		if len(row[0]) > width {
			width = len(row[0])
		}
	}
	for _, row := range rows {
		fmt.Fprintf(out, "%s  %s\n", labelStyle.Width(width).Render(row[0]), valueStyle.Render(row[1]))
	}
}

func emptyValue(value string) string {
	if value == "" {
		return mutedStyle.Render("none")
	}
	return value
}

func toolsValue(tools []string) string {
	if len(tools) == 0 {
		return mutedStyle.Render("none")
	}
	return strings.Join(tools, ", ")
}

func configValue(exists bool, path string) string {
	if !exists {
		return mutedStyle.Render("absent")
	}
	if path == "" {
		return okStyle.Render("present")
	}
	return okStyle.Render("present") + " " + path
}

func hashValue(path, hash string) string {
	if path == "" {
		return mutedStyle.Render("not available")
	}
	if hash == "" {
		return mutedStyle.Render("absent") + " " + path
	}
	return shortHash(hash) + " " + path
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func renderMarkdown(source string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(88),
	)
	if err != nil {
		return "", err
	}
	return renderer.Render(source)
}
