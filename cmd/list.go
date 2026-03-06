package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [pattern]",
	Short: "List notes in the vault",
	Long: `List notes in the vault, like ls for your notes.

  nora list                  all notes
  nora list -t               sort by modified time (newest first)
  nora list *.md             glob pattern
  nora list 2026-02*         notes from Feb 2026`,
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolP("time", "t", false, "Sort by modification time (newest first)")
	listCmd.Flags().BoolP("reverse", "R", false, "Reverse sort order")
}

func runList(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	sortByTime, _ := cmd.Flags().GetBool("time")
	reverse, _ := cmd.Flags().GetBool("reverse")

	pattern := "*.md"
	if len(args) > 0 {
		pattern = args[0]
		if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
			pattern = "*" + pattern + "*"
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read notes dir: %w", err)
	}

	type noteEntry struct {
		name    string
		modTime int64
	}
	var notes []noteEntry

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matched, _ := filepath.Match(pattern, e.Name())
		if !matched {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		notes = append(notes, noteEntry{e.Name(), info.ModTime().Unix()})
	}

	if sortByTime {
		sort.Slice(notes, func(i, j int) bool { return notes[i].modTime > notes[j].modTime })
	} else {
		sort.Slice(notes, func(i, j int) bool { return notes[i].name < notes[j].name })
	}

	if reverse {
		for i, j := 0, len(notes)-1; i < j; i, j = i+1, j-1 {
			notes[i], notes[j] = notes[j], notes[i]
		}
	}

	for _, n := range notes {
		fmt.Println(n.name)
	}

	return nil
}
