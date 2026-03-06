package cmd

import (
	"fmt"
	"strings"

	"n-notes/config"
	"n-notes/notes"

	"github.com/spf13/cobra"
)

var lookCmd = &cobra.Command{
	Use:   "look [query...]",
	Short: "Find a note and print it",
	Args:  cobra.ArbitraryArgs,
	RunE:  runLook,
}

var editCmd = &cobra.Command{
	Use:   "edit [query...]",
	Short: "Find a note and edit it in your editor",
	Args:  cobra.ArbitraryArgs,
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(lookCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.Flags().BoolP("look", "l", false, "Find a note and print it")
	rootCmd.Flags().BoolP("edit", "e", false, "Find a note and edit it")
}

func loadNotesDir() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("no config found — run `n setup` or create %s", config.Path())
	}
	return cfg.NotesDir, nil
}

func runLook(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}
	return notes.Look(dir, strings.Join(args, " "))
}

func runEdit(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}
	return notes.Open(dir, strings.Join(args, " "))
}
