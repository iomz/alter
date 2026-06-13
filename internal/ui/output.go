package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36"))
	okStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	warnStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	errStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))

	promptTheme = huh.ThemeBase()
)

func PrintRuntimeFound(out io.Writer, path string) {
	fmt.Fprintln(out, titleStyle.Render("mise runtime"))
	fmt.Fprintf(out, "%s %s\n", okStyle.Render("found"), path)
}

func PrintRuntimeMissing(out io.Writer, err error) {
	fmt.Fprintln(out, titleStyle.Render("mise runtime"))
	fmt.Fprintf(out, "%s %s\n", errStyle.Render("missing"), err)
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
		fmt.Fprintln(out, titleStyle.Render("setup mise"))
		fmt.Fprintln(out, strings.TrimSpace(body))
		return err
	}
	fmt.Fprint(out, rendered)
	return nil
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

func PrintStub(out io.Writer, title, body string) error {
	rendered, err := renderMarkdown("## " + title + "\n\n" + body)
	if err != nil {
		fmt.Fprintln(out, titleStyle.Render(title))
		fmt.Fprintln(out, strings.TrimSpace(body))
		return err
	}
	fmt.Fprint(out, rendered)
	return nil
}

func PrintPromptDeferred(out io.Writer) {
	fmt.Fprintln(out, promptTheme.Focused.Description.Render("interactive prompts: planned"))
}

func PrintRuntimeInstalled(out io.Writer, path string) {
	fmt.Fprintln(out, titleStyle.Render("mise runtime"))
	fmt.Fprintf(out, "%s %s\n", okStyle.Render("installed"), path)
}

func Warning(s string) string {
	return warnStyle.Render(s)
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
