package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"n-notes/ai"
	"n-notes/config"
	"n-notes/notes"

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
	} else {
		fmt.Println("Type your note (Ctrl-D to finish):")
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("could not read input: %w", err)
		}
		input = strings.TrimSpace(string(raw))
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

	formatted = stripCodeFences(formatted)
	filename := generateFilename(formatted)
	fullPath := filepath.Join(dir, filename)

	fmt.Println()
	fmt.Println(notes.Render(formatted))

	fmt.Printf("\nSave as %s? [y/N] ", filename)

	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(confirm)) != "y" {
		fmt.Println("Discarded.")
		return nil
	}

	if err := os.WriteFile(fullPath, []byte(formatted+"\n"), 0644); err != nil {
		return fmt.Errorf("could not save note: %w", err)
	}

	fmt.Printf("Saved to %s\n", fullPath)
	return nil
}

func generateFilename(content string) string {
	cfg, err := config.Load()
	if err != nil {
		// Fall back to defaults if config can't be loaded
		cfg = config.DefaultConfig()
	}

	title := extractTitle(content)
	date := extractDate(content)
	if date == "" {
		date = time.Now().Format(cfg.Format.DateFormat)
	}
	slug := slugify(title, cfg.Format.SlugStyle)

	name := cfg.Format.Naming
	name = strings.ReplaceAll(name, "{date}", date)
	name = strings.ReplaceAll(name, "{slug}", slug)

	return name + ".md"
}

func extractDate(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "date:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "date:"))
		}
	}
	return ""
}

func extractTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "title:") {
			title := strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			return strings.Trim(title, "\"'")
		}
	}
	return "note"
}

func slugify(s string, style string) string {
	switch style {
	case "snake":
		return slugifyWith(s, "_")
	case "pascal":
		return pascalize(s)
	default: // "kebab"
		return slugifyWith(s, "-")
	}
}

func slugifyWith(s string, sep string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, sep)
	s = strings.Trim(s, sep)
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, sep)
	}
	return s
}

func pascalize(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	words := reg.Split(s, -1)
	var result strings.Builder
	for _, w := range words {
		if w == "" {
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	out := result.String()
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i != -1 {
			s = s[i+1:]
		}
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimRight(s, "\n")
	}
	return s
}
