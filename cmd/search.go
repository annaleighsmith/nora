package cmd

import (
	"fmt"
	"strings"

	"github.com/annaleighsmith/nora/config"
	"github.com/annaleighsmith/nora/utils"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query...]",
	Short: "Search note contents with ripgrep",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

var showCmd = &cobra.Command{
	Use:   "show [query...]",
	Short: "Find a note and print it",
	Args:  cobra.ArbitraryArgs,
	RunE:  runShow,
}

var editCmd = &cobra.Command{
	Use:   "edit [query...]",
	Short: "Find a note and edit it in your editor",
	Args:  cobra.ArbitraryArgs,
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.Flags().BoolP("search", "s", false, "Search note contents")
	rootCmd.Flags().BoolP("show", "p", false, "Find a note and print it")
	rootCmd.Flags().BoolP("edit", "e", false, "Find a note and edit it")
}

func loadNotesDir() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("no config found — run `nora setup` or create %s", config.Path())
	}
	return cfg.NotesDir, nil
}

func runShow(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}
	return utils.Show(dir, strings.Join(args, " "))
}

func runEdit(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}
	return utils.Open(dir, strings.Join(args, " "))
}

func runSearch(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}
	query := strings.Join(args, " ")
	result, err := utils.SearchNotes(dir, query)
	if err != nil {
		return err
	}
	fmt.Print(result)
	return nil
}
