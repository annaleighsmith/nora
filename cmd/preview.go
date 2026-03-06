package cmd

import (
	"fmt"
	"os"

	"n-notes/notes"

	"github.com/spf13/cobra"
)

var previewCmd = &cobra.Command{
	Use:    "_preview [file]",
	Short:  "Render a note with glamour (used internally by fzf preview)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("could not read %s: %w", args[0], err)
		}
		fmt.Print(notes.Render(string(content)))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(previewCmd)
}
