package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
