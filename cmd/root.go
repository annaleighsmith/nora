package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/annaleighsmith/nora/ai"
	"github.com/annaleighsmith/nora/config"
	"github.com/annaleighsmith/nora/utils"

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
		cfg, err := config.Load()
		if err != nil {
			cfgPath, _ := config.Path() // best-effort path for error message
			return fmt.Errorf("no config found -- run `nora setup`. You can edit your config at %s", cfgPath)
		}
		ai.InitDebugLog(cfg.Debug)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, sf := range shortcutFlags {
			if v, _ := cmd.Flags().GetBool(sf.name); v {
				return sf.handler(cmd, args)
			}
		}
		return cmd.Help()
	},
}

// shortcutFlags maps root-level shortcut flags to their handler functions.
var shortcutFlags = []struct {
	name    string
	handler func(*cobra.Command, []string) error
}{
	{"search", runSearch},
	{"show", runShow},
	{"list", runList},
	{"edit", runEdit},
	{"add", runAdd},
	{"new", runNew},
}

var isTTY = term.IsTerminal(int(os.Stdout.Fd()))

func b(s string) string {
	if !isTTY {
		return s
	}
	return utils.Bold.Sprint(s)
}

func u(s string) string {
	if !isTTY {
		return s
	}
	return utils.Underline.Sprint(s)
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

func cleanup() {
	fmt.Fprintln(os.Stderr)
	utils.ClearLine(os.Stderr)
	utils.Dim.Fprintln(os.Stderr, "Goodbye!")
	ai.Usage.Print()
	ai.DebugLog.Close()
}

func Execute() {
	// Catch Ctrl+C for clean exit with usage + goodbye.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cleanup()
		os.Exit(0)
	}()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ai.Usage.Print()
	ai.DebugLog.Close()
}
