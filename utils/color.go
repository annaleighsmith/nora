package utils

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

var (
	// Dim is for secondary/status text (tool messages, hints, goodbye).
	Dim = color.New(color.Faint)

	// BoldCyan is for user input echo and prompt caret.
	BoldCyan = color.New(color.FgCyan, color.Bold)

	// Bold is for emphasis in help text.
	Bold = color.New(color.Bold)

	// Underline is for emphasis in help text.
	Underline = color.New(color.Underline, color.Bold)

	// Green is for success indicators.
	Green = color.New(color.FgGreen)

	// Red is for error indicators.
	Red = color.New(color.FgRed)
)

// Dimf prints a dim-formatted string to stderr.
// If a spinner is active, it clears the spinner line first and lets it redraw after.
func Dimf(format string, a ...any) {
	SpinnerAwarePrint(func() {
		Dim.Fprintf(os.Stderr, format, a...)
	})
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
