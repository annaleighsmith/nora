package cmd

import (
	"fmt"
	"os"

	"n-notes/config"

	"github.com/spf13/cobra"
)

// Commands that don't need a config file
var skipConfigCheck = map[string]bool{
	"setup":   true,
	"version": true,
	"help":    true,
}

var rootCmd = &cobra.Command{
	Use:   "n",
	Short: "A terminal-first note-taking tool",
	Args:  cobra.ArbitraryArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if skipConfigCheck[cmd.Name()] {
			return nil
		}
		if _, err := config.Load(); err != nil {
			return fmt.Errorf("no config found — run `n setup` or create %s", config.Path())
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		look, _ := cmd.Flags().GetBool("look")
		if look {
			return runLook(cmd, args)
		}
		edit, _ := cmd.Flags().GetBool("edit")
		if edit {
			return runEdit(cmd, args)
		}
		add, _ := cmd.Flags().GetBool("add")
		if add {
			return runAdd(cmd, args)
		}
		new, _ := cmd.Flags().GetBool("new")
		if new {
			return runNew(cmd, args)
		}
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
