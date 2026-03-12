package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	PromptHint  = Dim.Sprint("Continue chatting / exit to quit:")
	PromptCaret = BoldCyan.Sprint("> ")
)

// UserEcholn prints the user's input in bold cyan with the > prefix.
func UserEcholn(w io.Writer, input string) {
	BoldCyan.Fprintf(w, "> %s", input)
	fmt.Fprintln(w)
	fmt.Fprintln(w)
}

func promptHintFile(n int) string {
	return Dim.Sprintf("%d note(s) cited. [e]dit / [l]ook / continue chatting / exit to quit:", n)
}

func isExit(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	return s == "exit" || s == "/exit"
}

func goodbye() {
	Dim.Fprintln(os.Stderr, "Goodbye!")
}

// PromptFollowUp shows the continue prompt and reads a line.
// Returns the trimmed input and true if the session should end.
func PromptFollowUp(reader *bufio.Reader, citedFiles int) (string, bool) {
	if citedFiles > 0 {
		fmt.Println(promptHintFile(citedFiles))
	} else {
		fmt.Printf("\n%s\n", PromptHint)
	}
	fmt.Fprint(os.Stderr, PromptCaret)

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if isExit(input) {
		goodbye()
		return "", true
	}
	return input, false
}

// PromptBare shows just the caret and reads a line.
// Returns the trimmed input and true if the session should end.
func PromptBare(reader *bufio.Reader) (string, bool) {
	fmt.Fprint(os.Stderr, PromptCaret)

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if isExit(input) {
		goodbye()
		return "", true
	}
	return input, false
}
