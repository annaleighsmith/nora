package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const maxSearchLines = 100

// SearchNotes runs ripgrep against the notes directory and returns formatted results.
// If query starts with "#", it searches tags only in YAML frontmatter.
// Otherwise it does a general content search with context lines.
func SearchNotes(dir, query string) (string, error) {
	if query == "" {
		return "no query provided", nil
	}

	var args []string

	if strings.HasPrefix(query, "#") {
		tag := strings.TrimPrefix(query, "#")
		args = []string{
			"--no-messages",
			"--files-with-matches",
			fmt.Sprintf("tags:.*%s|^\\s*-\\s*%s", tag, tag),
			dir,
		}
	} else {
		args = []string{
			"--no-messages",
			"--context=2",
			"--max-count=5",
			"--heading",
			"--ignore-case",
			query,
			dir,
		}
	}

	cmd := exec.Command("rg", args...)
	out, err := cmd.Output()
	if err != nil {
		// rg exits 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "no matches found", nil
		}
		return "", fmt.Errorf("ripgrep error: %w", err)
	}

	result := strings.TrimSpace(string(out))
	if result == "" {
		return "no matches found", nil
	}

	return truncateResults(result), nil
}

// truncateResults caps output per file to maxSearchLines, appending a
// note about total file length so the AI can decide whether to read more.
func truncateResults(result string) string {
	// ripgrep --heading groups results by file, separated by blank lines
	sections := strings.Split(result, "\n\n")
	var out []string

	for _, section := range sections {
		lines := strings.Split(section, "\n")
		if len(lines) > maxSearchLines {
			truncated := lines[:maxSearchLines]
			// Count total lines in the actual file (first line is the filename in --heading mode)
			totalLines := countFileLines(strings.TrimSpace(lines[0]))
			truncated = append(truncated, fmt.Sprintf("... (truncated, file is %d lines total — use read_note to get more)", totalLines))
			out = append(out, strings.Join(truncated, "\n"))
		} else {
			out = append(out, section)
		}
	}

	return strings.Join(out, "\n\n")
}

func countFileLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(strings.Split(string(data), "\n"))
}

// ListTags scans all notes for YAML frontmatter tags and returns a sorted, deduplicated list.
func ListTags(notesDir string) (string, error) {
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		return "", fmt.Errorf("could not read notes dir: %w", err)
	}

	seen := make(map[string]int)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(notesDir, e.Name()))
		if err != nil {
			continue
		}
		_, tagsStr := extractFrontmatterMeta(string(data))
		if tagsStr == "" {
			continue
		}
		for _, t := range strings.Split(tagsStr, ", ") {
			t = strings.TrimSpace(t)
			if t != "" {
				seen[t]++
			}
		}
	}

	if len(seen) == 0 {
		return "no tags found", nil
	}

	type tagCount struct {
		tag   string
		count int
	}
	var tags []tagCount
	for t, c := range seen {
		tags = append(tags, tagCount{t, c})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].tag < tags[j].tag })

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d unique tags:\n", len(tags))
	for _, tc := range tags {
		fmt.Fprintf(&sb, "- %s (%d)\n", tc.tag, tc.count)
	}
	return sb.String(), nil
}

// ListRecentNotes returns the most recent notes by modification time.
func ListRecentNotes(notesDir string, count int) (string, error) {
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		return "", fmt.Errorf("could not read notes dir: %w", err)
	}

	type noteInfo struct {
		name    string
		modTime int64
	}
	var notes []noteInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		notes = append(notes, noteInfo{e.Name(), info.ModTime().Unix()})
	}

	sort.Slice(notes, func(i, j int) bool { return notes[i].modTime > notes[j].modTime })

	if count <= 0 || count > len(notes) {
		count = len(notes)
	}
	if count > 50 {
		count = 50
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d most recent notes (of %d total):\n", count, len(notes))
	for i := 0; i < count; i++ {
		fmt.Fprintf(&sb, "- %s\n", notes[i].name)
	}
	return sb.String(), nil
}

// NoteIndex returns a compact summary of all notes: filename, title, and tags.
func NoteIndex(notesDir string) (string, error) {
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		return "", fmt.Errorf("could not read notes dir: %w", err)
	}

	var sb strings.Builder
	noteCount := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		noteCount++
		data, err := os.ReadFile(filepath.Join(notesDir, e.Name()))
		if err != nil {
			continue
		}

		title, tags := extractFrontmatterMeta(string(data))
		line := e.Name()
		if title != "" {
			line += " | " + title
		}
		if tags != "" {
			line += " [" + tags + "]"
		}
		fmt.Fprintf(&sb, "%s\n", line)
	}

	header := fmt.Sprintf("%d notes in vault:\n", noteCount)
	return header + sb.String(), nil
}

// extractFrontmatterMeta pulls title and tags from YAML frontmatter.
func extractFrontmatterMeta(content string) (title, tags string) {
	if !strings.HasPrefix(content, "---") {
		return "", ""
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return "", ""
	}
	fm := content[3 : 3+end]

	var tagList []string
	inTags := false
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "title:") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "title:"))
			title = strings.Trim(title, "\"'")
		} else if strings.HasPrefix(trimmed, "tags:") {
			inTags = true
			// inline tags: tags: [a, b] or tags: a, b
			after := strings.TrimSpace(strings.TrimPrefix(trimmed, "tags:"))
			if after != "" {
				after = strings.Trim(after, "[]")
				for _, t := range strings.Split(after, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tagList = append(tagList, t)
					}
				}
				inTags = false
			}
		} else if inTags && strings.HasPrefix(trimmed, "-") {
			tag := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if tag != "" {
				tagList = append(tagList, tag)
			}
		} else if inTags && trimmed != "" {
			inTags = false
		}
	}

	return title, strings.Join(tagList, ", ")
}

// ReadNote reads a note by filename from the notes dir.
// If offset > 0, starts from that line. If limit > 0, returns at most that many lines.
// Returns the content plus a header with the total line count.
func ReadNote(notesDir, filename string, offset, limit int) (string, error) {
	clean := filepath.Base(filename)
	path := filepath.Join(notesDir, clean)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", clean, err)
	}

	lines := strings.Split(string(data), "\n")
	total := len(lines)

	if offset >= total {
		return fmt.Sprintf("[%s: %d lines total, offset %d is past end of file]", clean, total, offset), nil
	}

	if offset > 0 {
		lines = lines[offset:]
	}
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
	}

	header := fmt.Sprintf("[%s: %d lines total, showing lines %d-%d]", clean, total, offset+1, offset+len(lines))
	return header + "\n" + strings.Join(lines, "\n"), nil
}
