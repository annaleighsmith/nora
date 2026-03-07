package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	PromptHint = "\033[2mContinue chatting / exit to quit:\033[0m"
	PromptCaret = "\033[36;1m>\033[0m "
	UserEcho    = "\033[36;1m> %s\033[0m\n\n"

	promptHintFile = "\033[2m%d note(s) cited. [e]dit / [l]ook / continue chatting / exit to quit:\033[0m"
)

func isExit(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	return s == "exit" || s == "/exit"
}

func goodbye() {
	fmt.Fprintf(os.Stderr, "\033[2mGoodbye!\033[0m\n")
}

// PromptFollowUp shows the continue prompt and reads a line.
// Returns the trimmed input and true if the session should end.
func PromptFollowUp(reader *bufio.Reader, citedFiles int) (string, bool) {
	if citedFiles > 0 {
		fmt.Printf(promptHintFile+"\n", citedFiles)
	} else {
		fmt.Printf("\n" + PromptHint + "\n")
	}
	fmt.Fprintf(os.Stderr, PromptCaret)

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
	fmt.Fprintf(os.Stderr, PromptCaret)

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if isExit(input) {
		goodbye()
		return "", true
	}
	return input, false
}
