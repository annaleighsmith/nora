package cmd

import (
	"fmt"
	"sort"

	"github.com/annaleighsmith/nora/config"
)

func init() {
	rootCmd.Flags().Bool("model", false, "Manage AI models (list, get, set)")
}

func runModel(args []string) error {
	if len(args) == 0 {
		return runModelHelp()
	}

	switch args[0] {
	case "help", "--help", "-h":
		return runModelHelp()
	case "list":
		return runModelList()
	case "get":
		return runModelGet()
	case "set-light":
		return runModelSetTier("light", args[1:])
	case "set-heavy":
		return runModelSetTier("heavy", args[1:])
	case "set-all":
		return runModelSetTier("all", args[1:])
	default:
		return fmt.Errorf("unknown model command %q — use list, get, set-light, set-heavy, or set-all", args[0])
	}
}

func runModelHelp() error {
	fmt.Println(`Manage AI models

Usage:
  n --model <command> [args]

Commands:
  list                 Show available models
  get                  Show current model settings
  set-light <model>    Set model for light tasks (format, frontmatter)
  set-heavy <model>    Set model for heavy tasks (ask, query)
  set-all <model>      Set both light and heavy to the same model

Models can be aliases (haiku, sonnet, opus) or full model IDs.`)
	return nil
}

func runModelList() error {
	fmt.Println("Available models (alias → model ID):")
	// Sort aliases for stable output
	var aliases []string
	for alias := range config.ModelAliases {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		fmt.Printf("  %-10s %s\n", alias, config.ModelAliases[alias])
	}
	fmt.Println("\nYou can also use any full model ID directly.")
	return nil
}

func runModelGet() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	fmt.Printf("  light  %s  (%s)\n", cfg.Models.Light, config.ResolveModel(cfg.Models.Light))
	fmt.Printf("  heavy  %s  (%s)\n", cfg.Models.Heavy, config.ResolveModel(cfg.Models.Heavy))
	return nil
}

func runModelSetTier(tier string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: n --model %s <model>", "set-"+tier)
	}

	model := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch tier {
	case "light":
		cfg.Models.Light = model
	case "heavy":
		cfg.Models.Heavy = model
	case "all":
		cfg.Models.Light = model
		cfg.Models.Heavy = model
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Set %s → %s (%s)\n", tier, model, config.ResolveModel(model))
	return nil
}
