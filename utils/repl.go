package utils

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
)

var (
	PromptHint  = Dim.Render("Continue chatting / exit to quit:")
	PromptCaret = BoldCyan.Render("> ")
)

// UserEcholn prints the user's input in bold cyan with the > prefix.
func UserEcholn(w io.Writer, input string) {
	fmt.Fprint(w, BoldCyan.Render(fmt.Sprintf("> %s", input)))
	fmt.Fprintln(w)
	fmt.Fprintln(w)
}

func promptHintFile(n int) string {
	return Dim.Render(fmt.Sprintf("%d note(s) cited. [e]dit / [l]ook / continue chatting / exit to quit:", n))
}

func isExit(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	return s == "exit" || s == "/exit"
}

func goodbye() {
	fmt.Fprintln(os.Stderr, Dim.Render("Goodbye!"))
}

// PromptFollowUp shows the continue prompt and reads a line.
// Returns the trimmed input and true if the session should end.
func PromptFollowUp(citedFiles int) (string, bool) {
	if citedFiles > 0 {
		fmt.Println(promptHintFile(citedFiles))
	} else {
		fmt.Printf("\n%s\n", PromptHint)
	}

	return PromptBare()
}

// PromptBare shows just the caret and reads a line.
// Returns the trimmed input and true if the session should end.
func PromptBare() (string, bool) {
	if !IsInteractive() {
		fmt.Fprintln(os.Stderr, "error: interactive prompt requires a TTY on stdin")
		return "", true
	}

	var input string
	err := huh.NewInput().
		Prompt("> ").
		Value(&input).
		Run()
	if err != nil {
		goodbye()
		return "", true
	}
	input = strings.TrimSpace(input)
	if isExit(input) {
		goodbye()
		return "", true
	}
	return input, false
}
