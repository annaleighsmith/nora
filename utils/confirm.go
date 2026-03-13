package utils

import "github.com/charmbracelet/huh"

// Confirm shows a yes/no prompt using huh and returns the user's choice.
// Returns ErrNotInteractive if stdin is not a TTY.
// Returns (false, nil) on Ctrl-C.
func Confirm(prompt string) (bool, error) {
	if !IsInteractive() {
		return false, ErrNotInteractive
	}
	var v bool
	err := huh.NewConfirm().
		Title(prompt).
		Affirmative("Yes").
		Negative("No").
		Value(&v).
		Run()
	if err != nil {
		return false, nil
	}
	return v, nil
}
