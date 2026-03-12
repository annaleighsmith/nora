package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/annaleighsmith/nora/ai"
	"github.com/annaleighsmith/nora/config"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up nora with your notes directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		resetPrompts, _ := cmd.Flags().GetBool("reset-prompts")
		if resetPrompts {
			return runResetPrompts()
		}

		reader := bufio.NewReader(os.Stdin)

		defaultDir := config.DefaultConfig().NotesDir
		fmt.Printf("Where do you want to store your notes? [%s] ", defaultDir)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		notesDir := strings.TrimSpace(input)
		if notesDir == "" {
			notesDir = defaultDir
		}

		// Expand ~ to home directory
		if strings.HasPrefix(notesDir, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("could not determine home directory: %w", err)
			}
			notesDir = home + notesDir[1:]
		}

		// Check if directory exists
		if _, err := os.Stat(notesDir); os.IsNotExist(err) {
			fmt.Printf("Directory %s does not exist. Create it? [y/N] ", notesDir)
			confirm, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(confirm)) != "y" {
				fmt.Println("Setup cancelled.")
				return nil
			}
			if err := os.MkdirAll(notesDir, 0755); err != nil {
				return fmt.Errorf("could not create directory: %w", err)
			}
			fmt.Printf("Created %s\n", notesDir)
		}

		cfg := config.DefaultConfig()
		cfg.NotesDir = notesDir
		if err := config.Save(cfg); err != nil {
			return err
		}

		cfgPath, _ := config.Path()       // safe: Save() just succeeded
		promptsDir, _ := config.PromptsDir() // safe: home dir resolved above
		fmt.Printf("Config saved to %s\n", cfgPath)
		fmt.Printf("Prompts directory: %s\n", promptsDir)
		return nil
	},
}

// promptDefaults maps prompt names to their default content.
// Imported from the ai package constants.
var promptDefaults = map[string]string{
	"ask":         ai.DefaultAskPrompt,
	"manage":      ai.DefaultManagePrompt,
	"format":      ai.DefaultFormatPrompt,
	"frontmatter": ai.DefaultFrontmatterPrompt,
}

func runResetPrompts() error {
	dir, err := config.PromptsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create prompts directory: %w", err)
	}

	fmt.Println("Writing default prompts:")
	for name, content := range promptDefaults {
		path := filepath.Join(dir, name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("could not write %s: %w", name, err)
		}
		fmt.Printf("  %s.md\n", name)
	}

	fmt.Printf("\nPrompts reset to defaults at %s\n", dir)
	return nil
}

func init() {
	setupCmd.Flags().Bool("reset-prompts", false, "Reset all prompts to defaults")
	rootCmd.AddCommand(setupCmd)
}
