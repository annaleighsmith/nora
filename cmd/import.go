package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"n-notes/ai"
	"n-notes/config"

	"github.com/spf13/cobra"
)

// TODO: Consider accepting piped input (stdin) for file paths,
// e.g. `find ~/old-notes -name "*.md" | n import -`
var importCmd = &cobra.Command{
	Use:   "import <path>... [output-name.md]",
	Short: "Import markdown notes with AI formatting",
	Long: `Import .md files with AI-generated frontmatter. Accepts files and/or directories.

  nora import file.md                     single file, AI names it
  nora import file.md my-name.md          single file, you name it
  nora import a.md b.md c.md              multiple files
  nora import ~/old-notes/                directory (top-level only)
  nora import ~/old-notes/ -r             directory (recursive)
  nora import ~/old-notes/ -r -x Archive  recursive, skip Archive/`,
	Args: cobra.MinimumNArgs(1),
	RunE: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().BoolP("force", "f", false, "Re-import previously imported files")
	importCmd.Flags().BoolP("recursive", "r", false, "Recurse into subdirectories")
	importCmd.Flags().StringSliceP("exclude", "x", nil, "Directories to skip (repeatable, e.g. -x Archive -x Templates)")
}

type importEntry struct {
	originalPath string
	originalName string
	newFilename  string
	content      string
}

func runImport(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	recursive, _ := cmd.Flags().GetBool("recursive")
	ignoreDirs, _ := cmd.Flags().GetStringSlice("exclude")

	// Custom output name: n import file.md my-slug.md
	// Only when: exactly 2 args, first is an existing file, second doesn't exist on disk
	var customName string
	if len(args) == 2 {
		srcInfo, srcErr := os.Stat(args[0])
		_, destErr := os.Stat(args[1])
		if srcErr == nil && !srcInfo.IsDir() && os.IsNotExist(destErr) {
			candidate := args[1]
			if !strings.HasSuffix(candidate, ".md") {
				candidate += ".md"
			}
			customName = candidate
			args = args[:1]
		}
	}

	// Build ignore set (always skip dot directories)
	ignoreSet := make(map[string]bool)
	for _, d := range ignoreDirs {
		ignoreSet[d] = true
	}

	// Collect .md files from all args (files and directories)
	var matches []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return fmt.Errorf("cannot access %s: %w", arg, err)
		}
		if info.IsDir() && recursive {
			err := filepath.WalkDir(arg, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					name := d.Name()
					if strings.HasPrefix(name, ".") || ignoreSet[name] {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(path, ".md") {
					matches = append(matches, path)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("could not walk %s: %w", arg, err)
			}
		} else if info.IsDir() {
			dirMatches, err := filepath.Glob(filepath.Join(arg, "*.md"))
			if err != nil {
				return fmt.Errorf("could not glob %s: %w", arg, err)
			}
			matches = append(matches, dirMatches...)
		} else if strings.HasSuffix(arg, ".md") {
			matches = append(matches, arg)
		} else {
			fmt.Printf("Skipping %s (not a .md file)\n", arg)
		}
	}

	if len(matches) == 0 {
		fmt.Println("No .md files found.")
		return nil
	}

	fmt.Printf("Found %d markdown file(s)\n\n", len(matches))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	// Load the import ledger (previously imported paths)
	ledger := loadImportLedger()

	var entries []importEntry
	var skippedFormat []string
	var skippedAlready []string

	for i, path := range matches {
		name := filepath.Base(path)

		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		fmt.Printf("[%d/%d] Processing %s...\n", i+1, len(matches), name)

		if !force && ledger[absPath] {
			fmt.Printf("  SKIP (already imported): %s\n", name)
			skippedAlready = append(skippedAlready, name)
			continue
		}

		fileInfo, err := os.Stat(path)
		if err != nil {
			fmt.Printf("  WARNING: could not stat %s, skipping: %v\n", name, err)
			skippedFormat = append(skippedFormat, name)
			continue
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("  WARNING: could not read %s, skipping: %v\n", name, err)
			skippedFormat = append(skippedFormat, name)
			continue
		}

		input := strings.TrimSpace(string(raw))
		if input == "" {
			fmt.Printf("  WARNING: %s is empty, skipping\n", name)
			skippedFormat = append(skippedFormat, name)
			continue
		}

		fmt.Printf("  Generating frontmatter...\n")
		frontmatter, err := ai.GenerateFrontmatter(input, name)
		if err != nil {
			fmt.Printf("  WARNING: AI failed for %s, skipping: %v\n", name, err)
			skippedFormat = append(skippedFormat, name)
			continue
		}

		frontmatter = stripCodeFences(frontmatter)
		frontmatter = strings.TrimSpace(frontmatter)

		// Use the file's last modified date instead of today
		modDate := fileInfo.ModTime().Format(cfg.Format.DateFormat)
		frontmatter = replaceFrontmatterDate(frontmatter, modDate)

		// Extract inline #tags from note body and merge into frontmatter
		inlineTags := extractInlineTags(input)
		if len(inlineTags) > 0 {
			frontmatter = mergeTagsIntoFrontmatter(frontmatter, inlineTags)
		}

		// Prepend frontmatter to original content (preserved untouched)
		combined := frontmatter + "\n\n" + input
		var newFilename string
		if customName != "" {
			newFilename = deduplicateFilename(dir, generateFilenameFromSlug(frontmatter, customName))
		} else {
			newFilename = deduplicateFilename(dir, generateFilename(frontmatter))
		}

		entries = append(entries, importEntry{
			originalPath: absPath,
			originalName: name,
			newFilename:  newFilename,
			content:      combined,
		})
	}

	if len(entries) == 0 {
		fmt.Println("\nNo notes to import after formatting.")
		return nil
	}

	// Preview
	fmt.Println("\nReady to import:")
	for _, e := range entries {
		fmt.Printf("  %s -> %s\n", e.originalName, e.newFilename)
	}

	// Confirm
	fmt.Printf("\nImport %d note(s)? [y/N] ", len(entries))
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(confirm)) != "y" {
		fmt.Println("Import cancelled.")
		return nil
	}

	// Write files and record in ledger
	var imported int
	var importedPaths []string
	for _, e := range entries {
		dest := filepath.Join(dir, e.newFilename)
		if err := os.WriteFile(dest, []byte(e.content+"\n"), 0644); err != nil {
			fmt.Printf("  ERROR writing %s: %v\n", e.newFilename, err)
			continue
		}
		imported++
		importedPaths = append(importedPaths, e.originalPath)
	}

	if err := appendToImportLedger(importedPaths); err != nil {
		fmt.Printf("  WARNING: could not update import ledger: %v\n", err)
	}

	// Summary
	fmt.Printf("\nImported %d note(s) to %s\n", imported, dir)
	if len(skippedFormat) > 0 {
		fmt.Printf("Skipped (errors): %d\n", len(skippedFormat))
	}
	if len(skippedAlready) > 0 {
		fmt.Printf("Skipped (already imported): %d\n", len(skippedAlready))
	}

	return nil
}

func importLedgerPath() string {
	return filepath.Join(config.Dir(), "imported.txt")
}

// loadImportLedger reads the set of previously imported absolute paths.
func loadImportLedger() map[string]bool {
	ledger := make(map[string]bool)
	data, err := os.ReadFile(importLedgerPath())
	if err != nil {
		return ledger
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			ledger[line] = true
		}
	}
	return ledger
}

// appendToImportLedger adds newly imported paths to the ledger file.
func appendToImportLedger(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	f, err := os.OpenFile(importLedgerPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, p := range paths {
		if _, err := fmt.Fprintln(f, p); err != nil {
			return err
		}
	}
	return nil
}

// replaceFrontmatterDate swaps the date: value in YAML frontmatter with the
// given date string (e.g. the file's last modified date).
func replaceFrontmatterDate(frontmatter string, date string) string {
	lines := strings.Split(frontmatter, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "date:") {
			lines[i] = fmt.Sprintf("date: %s", date)
			return strings.Join(lines, "\n")
		}
	}
	return frontmatter
}

// extractInlineTags finds Obsidian-style #tags in note content, ignoring
// headings (## ), code blocks, and URLs.
var inlineTagRe = regexp.MustCompile(`(?:^|\s)#([a-zA-Z][a-zA-Z0-9_-]*)`)

func extractInlineTags(content string) []string {
	seen := make(map[string]bool)
	var tags []string
	inCodeBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Skip markdown headings
		if strings.HasPrefix(trimmed, "#") && (len(trimmed) == 1 || trimmed[1] == ' ' || trimmed[1] == '#') {
			continue
		}

		for _, match := range inlineTagRe.FindAllStringSubmatch(line, -1) {
			tag := strings.ToLower(match[1])
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// mergeTagsIntoFrontmatter adds inline tags to the existing tags list in the
// YAML frontmatter, deduplicating.
func mergeTagsIntoFrontmatter(frontmatter string, newTags []string) string {
	lines := strings.Split(frontmatter, "\n")

	// Find the tags line
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "tags:") {
			continue
		}

		// Parse existing tags from inline format: tags: [foo, bar]
		existing := make(map[string]bool)
		var existingTags []string

		if idx := strings.Index(trimmed, "["); idx != -1 {
			inner := trimmed[idx+1:]
			inner = strings.TrimSuffix(inner, "]")
			for _, t := range strings.Split(inner, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					existing[t] = true
					existingTags = append(existingTags, t)
				}
			}
		}

		// Merge new tags
		for _, t := range newTags {
			if !existing[t] {
				existing[t] = true
				existingTags = append(existingTags, t)
			}
		}

		lines[i] = fmt.Sprintf("tags: [%s]", strings.Join(existingTags, ", "))
		return strings.Join(lines, "\n")
	}

	// No tags line found — add one before the closing ---
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "---" {
			tagLine := fmt.Sprintf("tags: [%s]", strings.Join(newTags, ", "))
			lines = append(lines[:i], append([]string{tagLine}, lines[i:]...)...)
			return strings.Join(lines, "\n")
		}
	}

	return frontmatter
}

// deduplicateFilename appends -2, -3, etc. if a file with the same name
// already exists in dir. If skipName is provided, that name is not considered a conflict.
func deduplicateFilename(dir, filename string, skipName ...string) string {
	skip := ""
	if len(skipName) > 0 {
		skip = skipName[0]
	}

	if filename == skip {
		return filename
	}
	dest := filepath.Join(dir, filename)
	if _, err := os.Stat(dest); err != nil {
		return filename
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if candidate == skip {
			return candidate
		}
		if _, err := os.Stat(filepath.Join(dir, candidate)); err != nil {
			return candidate
		}
	}
	return filename
}
