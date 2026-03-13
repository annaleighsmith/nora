package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/annaleighsmith/nora/utils"

	"github.com/spf13/cobra"
)

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Fix common issues in notes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var fixFrontmatterCmd = &cobra.Command{
	Use:   "frontmatter",
	Short: "Fix broken or unfenced YAML frontmatter",
	RunE:  runFixFrontmatter,
}

var fixNamingCmd = &cobra.Command{
	Use:   "naming",
	Short: "Rename notes to match configured naming convention",
	RunE:  runFixNaming,
}

func init() {
	rootCmd.AddCommand(fixCmd)
	fixCmd.AddCommand(fixFrontmatterCmd)
	fixCmd.AddCommand(fixNamingCmd)
}

func runFixFrontmatter(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	broken, err := utils.FindBrokenFrontmatter(dir)
	if err != nil {
		return err
	}

	if len(broken) == 0 {
		fmt.Println("All notes have valid frontmatter fences.")
		return nil
	}

	fmt.Printf("Found %d note(s) with broken frontmatter:\n", len(broken))
	for _, name := range broken {
		fmt.Printf("  ~ %s\n", name)
	}

	ok, err := utils.Confirm("Fix these notes?")
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("Cancelled.")
		return nil
	}

	fixed := 0
	for _, name := range broken {
		path := filepath.Join(dir, name)
		changed, err := utils.FixFrontmatterFences(path)
		if err != nil {
			fmt.Printf("  error fixing %s: %v\n", name, err)
			continue
		}
		if changed {
			fixed++
			fmt.Printf("  ✓ %s\n", name)
		}
	}
	fmt.Printf("Fixed %d note(s).\n", fixed)
	return nil
}

func runFixNaming(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type rename struct {
		oldName string
		newName string
	}
	var renames []rename

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		expected := utils.GenerateFilename(string(data))
		if expected == e.Name() {
			continue
		}

		// Deduplicate if the expected name already exists (skip self)
		expected = deduplicateFilename(dir, expected, e.Name())
		if expected == e.Name() {
			continue
		}

		renames = append(renames, rename{e.Name(), expected})
	}

	if len(renames) == 0 {
		fmt.Println("All notes match the configured naming convention.")
		return nil
	}

	fmt.Printf("Found %d note(s) to rename:\n", len(renames))
	for _, r := range renames {
		fmt.Printf("  %s → %s\n", r.oldName, r.newName)
	}

	ok, err := utils.Confirm("Rename these notes?")
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("Cancelled.")
		return nil
	}

	renamed := 0
	for _, r := range renames {
		oldPath := filepath.Join(dir, r.oldName)
		newPath := filepath.Join(dir, r.newName)
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Printf("  error renaming %s: %v\n", r.oldName, err)
			continue
		}
		renamed++
		fmt.Printf("  ✓ %s → %s\n", r.oldName, r.newName)
	}
	fmt.Printf("Renamed %d note(s).\n", renamed)
	return nil
}
