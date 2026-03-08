package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/annaleighsmith/nora/utils"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete or archive notes",
	Long: `Delete or archive notes with interactive fzf multi-select.

  nora delete              Interactive fzf multi-select
  nora delete -A           Archive instead of delete

For AI-assisted deletion, use: nora manage`,
	Args: cobra.NoArgs,
	RunE: runDelete,
}

func init() {
	deleteCmd.Flags().BoolP("archive", "A", false, "Move to .archive/ instead of deleting")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	archive, _ := cmd.Flags().GetBool("archive")

	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("No notes found.")
		return nil
	}

	selected, err := utils.PickMultiFrom(files, false)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return nil
	}

	return confirmAndDelete(selected, dir, archive)
}

func confirmAndDelete(files []string, dir string, archive bool) error {
	action := "Delete"
	if archive {
		action = "Archive"
	}

	fmt.Println()
	for _, f := range files {
		fmt.Printf("  %s\n", filepath.Base(f))
	}

	fmt.Printf("\n%s %d note(s)? [y/N] ", action, len(files))
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "y" && input != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	if archive {
		archiveDir := filepath.Join(dir, ".archive")
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			return fmt.Errorf("could not create .archive directory: %w", err)
		}
		for _, f := range files {
			dest := filepath.Join(archiveDir, filepath.Base(f))
			if err := os.Rename(f, dest); err != nil {
				fmt.Fprintf(os.Stderr, "Could not archive %s: %v\n", filepath.Base(f), err)
				continue
			}
			fmt.Printf("  Archived %s\n", filepath.Base(f))
		}
	} else {
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				fmt.Fprintf(os.Stderr, "Could not delete %s: %v\n", filepath.Base(f), err)
				continue
			}
			fmt.Printf("  Deleted %s\n", filepath.Base(f))
		}
	}

	return nil
}
