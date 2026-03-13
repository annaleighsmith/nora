package utils

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var (
	// Dim is for secondary/status text (tool messages, hints, goodbye).
	Dim = lipgloss.NewStyle().Faint(true)

	// BoldCyan is for user input echo and prompt caret.
	BoldCyan = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)

	// Bold is for emphasis in help text.
	Bold = lipgloss.NewStyle().Bold(true)

	// Underline is for emphasis in help text.
	Underline = lipgloss.NewStyle().Underline(true).Bold(true)

	// Green is for success indicators.
	Green = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	// BoldGreen is for note titles in chrome.
	BoldGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)

	// Red is for error indicators.
	Red = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	// DimCyan is for tags in note chrome.
	DimCyan = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Faint(true)
)

// Dimf prints a dim-formatted string to stderr.
// If a spinner is active, it clears the spinner line first and lets it redraw after.
func Dimf(format string, a ...any) {
	SpinnerAwarePrint(func() {
		fmt.Fprint(os.Stderr, Dim.Render(fmt.Sprintf(format, a...)))
	})
}

// highlightRenderer forces ANSI color output regardless of TTY detection,
// so styles work in non-TTY contexts like fzf preview panes.
var highlightRenderer = func() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(os.Stdout)
	r.SetColorProfile(termenv.ANSI)
	return r
}()

// highlightStyle is bold red, rendered via the forced-color renderer.
var highlightStyle = highlightRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))

// HighlightMatches wraps case-insensitive occurrences of query in bold red.
// Uses a forced-color renderer so it works in non-TTY contexts (e.g. fzf preview).
func HighlightMatches(text, query string) string {
	if query == "" {
		return text
	}
	lower := strings.ToLower(text)
	q := strings.ToLower(query)

	var sb strings.Builder
	i := 0
	for {
		idx := strings.Index(lower[i:], q)
		if idx == -1 {
			sb.WriteString(text[i:])
			break
		}
		sb.WriteString(text[i : i+idx])
		sb.WriteString(highlightStyle.Render(text[i+idx : i+idx+len(q)]))
		i += idx + len(q)
	}
	return sb.String()
}

// ClearLine erases the current terminal line.
func ClearLine(w io.Writer) {
	fmt.Fprintf(w, "\r\033[2K")
}

// ClearLinesUp moves cursor up n lines and clears each one.
func ClearLinesUp(w io.Writer, n int) {
	for range n {
		fmt.Fprintf(w, "\033[A\033[2K")
	}
	fmt.Fprintf(w, "\r")
}
