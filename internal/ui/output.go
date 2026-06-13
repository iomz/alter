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
