package cmd

import (
	"fmt"
	"sort"

	"github.com/annaleighsmith/nora/config"

	"github.com/spf13/cobra"
)

var validProviders = []string{"anthropic", "claude-code"}

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage AI models and provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show available models and providers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available models (alias -> model ID):")
		var aliases []string
		for alias := range config.ModelAliases {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		for _, alias := range aliases {
			fmt.Printf("  %-10s %s\n", alias, config.ModelAliases[alias])
		}
		fmt.Println("\nYou can also use any full model ID directly.")

		fmt.Println("\nAvailable providers:")
		for _, p := range validProviders {
			fmt.Printf("  %s\n", p)
		}
		return nil
	},
}

var modelGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show current model settings",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("  provider  %s\n", cfg.Models.Provider)
		fmt.Printf("  light     %s  (%s)\n", cfg.Models.Light, config.ResolveModel(cfg.Models.Light))
		fmt.Printf("  heavy     %s  (%s)\n", cfg.Models.Heavy, config.ResolveModel(cfg.Models.Heavy))
		return nil
	},
}

var modelSetLightCmd = &cobra.Command{
	Use:   "set-light <model>",
	Short: "Set model for light tasks (format, frontmatter)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setModelTier("light", args[0])
	},
}

var modelSetHeavyCmd = &cobra.Command{
	Use:   "set-heavy <model>",
	Short: "Set model for heavy tasks (ask, query)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setModelTier("heavy", args[0])
	},
}

var modelSetAllCmd = &cobra.Command{
	Use:   "set-all <model>",
	Short: "Set both light and heavy to the same model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setModelTier("all", args[0])
	},
}

var modelSetProviderCmd = &cobra.Command{
	Use:   "set-provider <name>",
	Short: "Set AI backend (anthropic, claude-code)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		valid := false
		for _, p := range validProviders {
			if name == p {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unknown provider %q — valid options: %v", name, validProviders)
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		cfg.Models.Provider = name
		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("Set provider -> %s\n", name)

		if name == "claude-code" {
			fmt.Println("\n  Warning: Claude Code Backend")
			fmt.Println("  This option only exists because this is a non-commercial,")
			fmt.Println("  low-volume, open source CLI application. Use at your own")
			fmt.Println("  risk and check Anthropic's latest policy.")
		}

		return nil
	},
}

func setModelTier(tier, model string) error {
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

	fmt.Printf("Set %s -> %s (%s)\n", tier, model, config.ResolveModel(model))
	return nil
}

func init() {
	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelGetCmd)
	modelCmd.AddCommand(modelSetLightCmd)
	modelCmd.AddCommand(modelSetHeavyCmd)
	modelCmd.AddCommand(modelSetAllCmd)
	modelCmd.AddCommand(modelSetProviderCmd)
	rootCmd.AddCommand(modelCmd)
}
