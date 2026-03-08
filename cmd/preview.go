package cmd

import (
	"fmt"
	"os"

	"github.com/annaleighsmith/nora/utils"

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
		fmt.Print(utils.Render(string(content)))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(previewCmd)
}
