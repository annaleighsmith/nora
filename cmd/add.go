package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/annaleighsmith/nora/ai"
	"github.com/annaleighsmith/nora/utils"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Quick note — type in terminal, AI formats",
	RunE:  runAdd,
}

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "New note — write in your editor, AI formats",
	RunE:  runNew,
}

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.Flags().BoolP("add", "a", false, "Quick note — type in terminal, AI formats")
	rootCmd.Flags().BoolP("new", "n", false, "New note — write in editor, AI formats")
}

func runAdd(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	var input string
	if len(args) > 0 {
		input = strings.Join(args, " ")
	} else if !utils.IsInteractive() {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("could not read stdin: %w", err)
		}
		input = strings.TrimSpace(string(raw))
	} else {
		err := huh.NewText().
			Title("Type your note").
			Value(&input).
			Run()
		if err != nil {
			return nil
		}
		input = strings.TrimSpace(input)
	}

	return formatAndSave(dir, input)
}

func runNew(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	// Create a temp file for editing
	tmp, err := os.CreateTemp("", "n-note-*.md")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	// Open in editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}
	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("could not read temp file: %w", err)
	}

	return formatAndSave(dir, strings.TrimSpace(string(raw)))
}

func formatAndSave(dir, input string) error {
	if input == "" {
		fmt.Println("Empty input, nothing to add.")
		return nil
	}

	fmt.Println("\nFormatting with Claude...")
	formatted, err := ai.Format(input, "")
	if err != nil {
		return err
	}

	formatted = utils.StripCodeFences(formatted)
	filename := utils.GenerateFilename(formatted)
	fullPath := filepath.Join(dir, filename)

	fmt.Println()
	fmt.Println(utils.Render(formatted))

	if utils.IsInteractive() {
		ok, err := utils.Confirm(fmt.Sprintf("Save as %s?", filename))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Discarded.")
			return nil
		}
	}

	if err := os.WriteFile(fullPath, []byte(formatted+"\n"), 0644); err != nil {
		return fmt.Errorf("could not save note: %w", err)
	}

	fmt.Printf("Saved to %s\n", fullPath)
	return nil
}

