package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
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
	flag    string
	handler func(*cobra.Command, []string) error
}{
	{"search", "s", runSearch},
	{"show", "p", runShow},
	{"list", "l", runList},
	{"edit", "e", runEdit},
	{"add", "a", runAdd},
	{"new", "n", runNew},
}

var isTTY = term.IsTerminal(int(os.Stdout.Fd()))

func b(s string) string {
	if !isTTY {
		return s
	}
	return utils.Bold.Render(s)
}

func u(s string) string {
	if !isTTY {
		return s
	}
	return utils.Underline.Render(s)
}

// commandHelp returns the formatted Commands section from registered subcommands.
func commandHelp() string {
	var cmds []*cobra.Command
	maxLen := 0
	for _, c := range rootCmd.Commands() {
		if c.Hidden {
			continue
		}
		cmds = append(cmds, c)
		if len(c.Name()) > maxLen {
			maxLen = len(c.Name())
		}
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name() < cmds[j].Name() })

	var sb strings.Builder
	for _, c := range cmds {
		pad := strings.Repeat(" ", maxLen-len(c.Name()))
		fmt.Fprintf(&sb, "  %s%s  %s\n", b(c.Name()), pad, c.Short)
	}
	return sb.String()
}

// shortcutHelp returns the formatted Shortcut Flags section.
func shortcutHelp() string {
	sorted := make([]struct{ name, flag string }, len(shortcutFlags))
	for i, sf := range shortcutFlags {
		sorted[i] = struct{ name, flag string }{sf.name, sf.flag}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].flag < sorted[j].flag })

	var sb strings.Builder
	for _, sf := range sorted {
		fmt.Fprintf(&sb, "  %s         Same as nora %s\n", b("-"+sf.flag), sf.name)
	}
	return sb.String()
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

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
%s
%s
%s
%s
  -h, --help     Print help
  -V, --version  Print version
`,
			u(b("nora v0.1.0")),
			b("Usage:"),
			b("Commands:"),
			commandHelp(),
			b("Shortcut Flags:"),
			shortcutHelp(),
			b("Options:"),
		)
	})
}

func cleanup() {
	fmt.Fprintln(os.Stderr)
	utils.ClearLine(os.Stderr)
	fmt.Fprintln(os.Stderr, utils.Dim.Render("Goodbye!"))
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
