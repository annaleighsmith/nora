package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"n-notes/config"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up n with your notes directory for nnotes",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			home, _ := os.UserHomeDir()
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

		fmt.Printf("Config saved to %s\n", config.Path())
		fmt.Printf("Prompts directory: %s\n", config.PromptsDir())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
