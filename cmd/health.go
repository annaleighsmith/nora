package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"n-notes/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check config, deps, and API connectivity",
	RunE:  runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

var tw *tabwriter.Writer

func runHealth(cmd *cobra.Command, args []string) error {
	ok := true
	tw = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Config
	cfg, err := config.Load()
	if err != nil {
		printStatus("Config", false, err.Error())
		return nil
	}
	printStatus("Config", true, config.Path())

	// Notes dir
	dir := expandHome(cfg.NotesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		printStatus("Notes dir", false, fmt.Sprintf("%s — %v", dir, err))
		ok = false
	} else {
		mdCount := 0
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				mdCount++
			}
		}
		printStatus("Notes dir", true, fmt.Sprintf("%s (%d notes)", dir, mdCount))
	}

	// Models
	printStatus("Light model", true, config.ResolveModel(cfg.Models.Light))
	printStatus("Heavy model", true, config.ResolveModel(cfg.Models.Heavy))

	// Bot
	if cfg.Bot.Name != "" {
		printStatus("Bot", true, fmt.Sprintf("%s — %s", cfg.Bot.Name, cfg.Bot.Personality))
	} else {
		printStatus("Bot", true, "unnamed (set [bot] name in config)")
	}

	// Read budget
	printStatus("Read budget", true, fmt.Sprintf("%d lines/question", cfg.Bot.AskReadBudget))

	// Dependencies
	for _, dep := range []struct {
		name string
		flag string
	}{
		{"rg", "--version"},
		{"fzf", "--version"},
		{"git", "--version"},
	} {
		out, err := exec.Command(dep.name, dep.flag).Output()
		if err != nil {
			printStatus(dep.name, false, "not found")
			ok = false
		} else {
			version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
			printStatus(dep.name, true, version)
		}
	}

	// API key
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		printStatus("API key", false, "ANTHROPIC_API_KEY not set")
		ok = false
	} else {
		masked := key[:8] + "..." + key[len(key)-4:]
		printStatus("API key", true, masked)
	}

	// API probe
	if key != "" {
		model := anthropic.Model(config.ResolveModel(cfg.Models.Light))
		client := anthropic.NewClient()

		start := time.Now()
		resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
			Model:     model,
			MaxTokens: 4,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock("hi")),
			},
		})
		elapsed := time.Since(start)

		if err != nil {
			printStatus("API probe", false, err.Error())
			ok = false
		} else {
			printStatus("API probe", true, fmt.Sprintf("%s responded in %dms (%d input + %d output tokens)",
				string(model), elapsed.Milliseconds(), resp.Usage.InputTokens, resp.Usage.OutputTokens))
		}
	}

	// Memories
	memPath := filepath.Join(config.Dir(), "memories.md")
	if data, err := os.ReadFile(memPath); err == nil {
		count := 0
		for _, line := range strings.Split(string(data), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "•") {
				count++
			}
		}
		printStatus("Memories", true, fmt.Sprintf("%d stored", count))
	} else {
		printStatus("Memories", true, "none yet")
	}

	tw.Flush()
	fmt.Println()
	if ok {
		fmt.Println("All systems go.")
	} else {
		fmt.Println("Some issues found — see above.")
	}
	return nil
}

func printStatus(label string, ok bool, detail string) {
	icon := "\033[32m✓\033[0m"
	if !ok {
		icon = "\033[31m✗\033[0m"
	}
	fmt.Fprintf(tw, "  %s\t%s\t%s\n", icon, label, detail)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
