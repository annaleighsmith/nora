package cmd

import (
	"fmt"
	"os"

	"github.com/annaleighsmith/nora/ai"
	"github.com/annaleighsmith/nora/config"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Commands that don't need a config file
var skipConfigCheck = map[string]bool{
	"setup":   true,
	"version": true,
	"health":  true,
	"help":    true,
}

var rootCmd = &cobra.Command{
	Use:   "nora",
	Short: "A terminal-first AI-powered note-taking tool",
	Args:  cobra.ArbitraryArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if skipConfigCheck[cmd.Name()] {
			return nil
		}
		if _, err := config.Load(); err != nil {
			return fmt.Errorf("no config found -- run `nora setup`. You can edit your config at %s", config.Path())
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		model, _ := cmd.Flags().GetBool("model")
		if model {
			return runModel(args)
		}
		search, _ := cmd.Flags().GetBool("search")
		if search {
			return runSearch(cmd, args)
		}
		show, _ := cmd.Flags().GetBool("show")
		if show {
			return runShow(cmd, args)
		}
		list, _ := cmd.Flags().GetBool("list")
		if list {
			return runList(cmd, args)
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

var isTTY = term.IsTerminal(int(os.Stdout.Fd()))

func b(s string) string {
	if !isTTY {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func u(s string) string {
	if !isTTY {
		return s
	}
	return "\033[4m" + s + "\033[0m"
}

func init() {
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != rootCmd {
			fmt.Fprint(os.Stdout, cmd.UsageString())
			return
		}
		fmt.Fprintf(os.Stdout, `
%s
Anna Smith <annaleighsmith60@gmail.com>
https://github.com/annaleighsmith

A terminal-first AI-powered note-taking tool

%s
  nora <COMMAND>
  nora [FLAGS] [input]

%s
  %s        Quick note — type in terminal, AI formats
  %s        Ask a question about your notes (-f for file scope)
  %s     Delete or archive notes (fzf)
  %s       Find a note and edit it in your editor
  %s        Fix common issues in notes
  %s     Check config, deps, and API connectivity
  %s     Import markdown notes with AI formatting
  %s       List notes in the vault (alias: ls)
  %s     Manage your vault with AI (read + write)
  %s      Manage AI models (list, get, set)
  %s        New note — write in your editor, AI formats
  %s     Search note contents with ripgrep
  %s      Set up nora with your notes directory
  %s       Find a note and print it
  %s       List or manage tags

%s
  %s         Quick add (same as nora add)
  %s         Edit (same as nora edit)
  %s         List (same as nora list)
  %s         New (same as nora new)
  %s         Print (same as nora show)
  %s         Search (same as nora search)

%s
  -h, --help     Print help
  -V, --version  Print version
`,
			u(b("nora v0.1.0")),
			b("Usage:"),
			b("Commands:"),
			b("add"), b("ask"), b("delete"), b("edit"), b("fix"),
			b("health"), b("import"), b("list"), b("manage"),
			b("model"), b("new"), b("search"),
			b("setup"), b("show"), b("tags"),
			b("Shortcut Flags:"),
			b("-a"), b("-e"), b("-l"), b("-n"), b("-p"), b("-s"),
			b("Options:"),
		)
	})
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ai.Usage.Print()
}
