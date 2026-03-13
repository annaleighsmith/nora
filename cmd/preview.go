package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/annaleighsmith/nora/utils"

	"github.com/spf13/cobra"
)

var previewCmd = &cobra.Command{
	Use:    "_preview [file] [query]",
	Short:  "Render a note for fzf preview (used internally)",
	Hidden: true,
	Args:   cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// args[0] may be a tab-delimited line (path\tfilename\t...); extract the path
		filePath := strings.TrimSpace(args[0])
		if i := strings.Index(filePath, "\t"); i != -1 {
			filePath = filePath[:i]
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("could not read %q: %w", filePath, err)
		}

		raw := string(content)
		filename := filepath.Base(filePath)
		meta := utils.ParseNoteMeta(raw, filename)

		// Header: filename, date, tags on separate lines
		fmt.Println(filename)
		if meta.Date != "" {
			fmt.Println(meta.Date)
		}
		if meta.Tags != "" {
			fmt.Println(meta.Tags)
		}
		fmt.Println()

		// Body: strip frontmatter, render without color
		width := 80
		if cols, err := strconv.Atoi(os.Getenv("FZF_PREVIEW_COLUMNS")); err == nil && cols > 0 {
			width = cols
		}
		body := utils.StripFrontmatter(raw)
		rendered := utils.RenderWidth(body, width)
		plain := utils.StripANSI(rendered)

		// Highlight query matches if provided
		query := ""
		if len(args) > 1 {
			query = args[1]
		}
		if query != "" {
			plain = utils.HighlightMatches(plain, query)
		}

		fmt.Print(plain)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(previewCmd)
}
