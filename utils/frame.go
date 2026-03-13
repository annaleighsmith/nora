package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// ErrNotInteractive is returned when a command requires a TTY on stdin
// but none is available (e.g. piped input, AI agent invocation).
var ErrNotInteractive = errors.New("this command requires an interactive terminal")

// IsInteractive returns true if stdin is a terminal (i.e. the user can type).
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// BoxStyle defines the characters used to draw a box frame.
type BoxStyle struct {
	TopLeft, TopRight, BottomLeft, BottomRight string
	Horizontal, Vertical                       string
	LeftT, RightT                              string
}

// RoundedBox uses rounded Unicode box-drawing characters.
var RoundedBox = BoxStyle{
	TopLeft:     "╭",
	TopRight:    "╮",
	BottomLeft:  "╰",
	BottomRight: "╯",
	Horizontal:  "─",
	Vertical:    "│",
	LeftT:       "├",
	RightT:      "┤",
}

// NoteMeta holds parsed frontmatter fields for display in note chrome.
type NoteMeta struct {
	Filename string
	Date     string
	Tags     string // comma-separated
}

// ParseNoteMeta extracts filename, date, and tags from note content.
func ParseNoteMeta(content, filename string) NoteMeta {
	meta := NoteMeta{Filename: filename}
	fm, _ := ParseFrontmatter(content)

	meta.Date = fm.Date
	// Trim to just the date portion (no time)
	if i := strings.Index(meta.Date, "T"); i != -1 {
		meta.Date = meta.Date[:i]
	}
	if i := strings.Index(meta.Date, " "); i != -1 {
		meta.Date = meta.Date[:i]
	}

	meta.Tags = strings.Join(fm.Tags, ", ")
	return meta
}

// StripFrontmatter removes the YAML frontmatter block (---...---) from content.
func StripFrontmatter(content string) string {
	_, body := splitFrontmatter(content)
	return body
}

// IsTTY returns true if stdout is a terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// TermWidth returns the current terminal width, defaulting to 80.
func TermWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

// FrameContent wraps rendered body text in a bordered box with metadata chrome.
func FrameContent(body string, meta NoteMeta) string {
	style := RoundedBox
	width := TermWidth()
	if width < 30 {
		width = 30
	}

	// Inner width = total width - 2 (left/right border) - 4 (2 padding spaces each side)
	innerWidth := width - 6

	dim := Dim.Render
	dimCyan := DimCyan.Render

	var sb strings.Builder

	// Top bar: ╭─ filename ─────╮
	fileLabel := " " + meta.Filename + " "
	fileLabelLen := utf8.RuneCountInString(fileLabel)
	// "╭─" = 2 chars + fileLabel + fill + "╮" = 1 char → fill = width - 3 - label
	topFill := width - 3 - fileLabelLen
	if topFill < 1 {
		topFill = 1
	}
	sb.WriteString(dim(style.TopLeft + style.Horizontal))
	boldGreen := BoldGreen.Render
	sb.WriteString(boldGreen(fileLabel))
	sb.WriteString(dim(strings.Repeat(style.Horizontal, topFill) + style.TopRight))
	sb.WriteString("\n")

	// Meta row (date + tags) — skip if both empty
	hasMeta := meta.Date != "" || meta.Tags != ""
	if hasMeta {
		var metaParts []string
		if meta.Date != "" {
			metaParts = append(metaParts, meta.Date)
		}
		if meta.Tags != "" {
			// If tags are pre-styled (contain ANSI), use as-is
			if strings.Contains(meta.Tags, "\x1b[") {
				metaParts = append(metaParts, meta.Tags)
			} else {
				metaParts = append(metaParts, dimCyan(meta.Tags))
			}
		}

		metaText := strings.Join(metaParts, "  ")
		metaVisLen := visibleLen(metaText)
		metaPad := innerWidth - metaVisLen
		if metaPad < 0 {
			metaPad = 0
		}

		sb.WriteString(dim(style.Vertical))
		sb.WriteString("  " + metaText + strings.Repeat(" ", metaPad) + "  ")
		sb.WriteString(dim(style.Vertical))
		sb.WriteString("\n")

		// Separator: ├──────┤
		sb.WriteString(dim(style.LeftT + strings.Repeat(style.Horizontal, width-2) + style.RightT))
		sb.WriteString("\n")
	}

	// Body lines
	bodyLines := strings.Split(body, "\n")
	// Trim trailing empty lines from glamour output
	for len(bodyLines) > 0 && strings.TrimSpace(bodyLines[len(bodyLines)-1]) == "" {
		bodyLines = bodyLines[:len(bodyLines)-1]
	}

	for _, line := range bodyLines {
		vLen := visibleLen(line)
		if vLen > innerWidth {
			line = truncateToWidth(line, innerWidth)
			vLen = innerWidth
		}
		pad := innerWidth - vLen
		sb.WriteString(dim(style.Vertical))
		sb.WriteString("  " + line + strings.Repeat(" ", pad) + "  ")
		sb.WriteString(dim(style.Vertical))
		sb.WriteString("\n")
	}

	// Bottom: ╰──────╯
	sb.WriteString(dim(style.BottomLeft + strings.Repeat(style.Horizontal, width-2) + style.BottomRight))
	sb.WriteString("\n")

	return sb.String()
}

// FrameWarning wraps a warning title and body in a bordered box.
func FrameWarning(title, body string) string {
	style := RoundedBox
	width := TermWidth()
	if width < 30 {
		width = 30
	}

	innerWidth := width - 6
	dim := Dim.Render
	redBold := Red.Bold(true).Render

	var sb strings.Builder

	// Top bar: ╭─ Warning: ... ─────╮
	titleLabel := " " + title + " "
	titleLabelLen := utf8.RuneCountInString(titleLabel)
	topFill := width - 3 - titleLabelLen
	if topFill < 1 {
		topFill = 1
	}
	sb.WriteString(dim(style.TopLeft + style.Horizontal))
	sb.WriteString(redBold(titleLabel))
	sb.WriteString(dim(strings.Repeat(style.Horizontal, topFill) + style.TopRight))
	sb.WriteString("\n")

	// Body lines
	for _, line := range strings.Split(body, "\n") {
		vLen := visibleLen(line)
		if vLen > innerWidth {
			line = truncateToWidth(line, innerWidth)
			vLen = innerWidth
		}
		pad := innerWidth - vLen
		sb.WriteString(dim(style.Vertical))
		sb.WriteString("  " + dim(line) + strings.Repeat(" ", pad) + "  ")
		sb.WriteString(dim(style.Vertical))
		sb.WriteString("\n")
	}

	// Bottom: ╰──────╯
	sb.WriteString(dim(style.BottomLeft + strings.Repeat(style.Horizontal, width-2) + style.BottomRight))
	sb.WriteString("\n")

	return sb.String()
}

// FrameSearchResults wraps ripgrep --heading output into framed cards.
// Each heading section becomes a card with filename, date/tags chrome, and
// match lines re-highlighted with our own bold-red style.
func FrameSearchResults(raw, notesDir, query string) string {
	if raw == "" || raw == "no matches found" || raw == "no query provided" {
		return raw + "\n"
	}

	// Strip leading # for tag searches when highlighting
	highlightQuery := query
	if strings.HasPrefix(highlightQuery, "#") {
		highlightQuery = strings.TrimPrefix(highlightQuery, "#")
	}

	// Split into per-file sections by detecting file path headings.
	// Can't split on \n\n because context lines may contain blank lines.
	type section struct {
		path string
		body []string
	}
	var sections []section

	for _, line := range strings.Split(raw, "\n") {
		stripped := StripANSI(strings.TrimSpace(line))
		if strings.HasPrefix(stripped, notesDir) && strings.HasSuffix(stripped, ".md") {
			sections = append(sections, section{path: stripped})
		} else if len(sections) > 0 {
			sections[len(sections)-1].body = append(sections[len(sections)-1].body, line)
		}
	}

	var framed []string
	for _, sec := range sections {
		filename := filepath.Base(sec.path)

		// Read actual note to get metadata + frontmatter boundary
		content, err := os.ReadFile(sec.path)
		var meta NoteMeta
		var fmEnd int
		if err == nil {
			meta = ParseNoteMeta(string(content), filename)
			fmEnd = FrontmatterEndLine(string(content))
		} else {
			meta = NoteMeta{Filename: filename}
		}

		// Bold matching tags in meta row
		if meta.Tags != "" {
			meta.Tags = styleSearchTags(meta.Tags, highlightQuery)
		}

		// Strip ripgrep ANSI, filter frontmatter lines, strip line numbers, re-highlight
		var bodyLines []string
		for _, line := range sec.body {
			clean := StripANSI(line)
			lineNum := ripgrepLineNum(clean)
			if fmEnd > 0 && lineNum > 0 && lineNum <= fmEnd {
				continue
			}
			clean = stripRipgrepPrefix(clean)
			bodyLines = append(bodyLines, HighlightMatches(clean, highlightQuery))
		}

		// Trim leading/trailing empty lines first
		for len(bodyLines) > 0 && strings.TrimSpace(bodyLines[0]) == "" {
			bodyLines = bodyLines[1:]
		}
		for len(bodyLines) > 0 && strings.TrimSpace(bodyLines[len(bodyLines)-1]) == "" {
			bodyLines = bodyLines[:len(bodyLines)-1]
		}

		// Clean up -- separators (leading, trailing, consecutive)
		var cleaned []string
		for _, line := range bodyLines {
			if strings.TrimSpace(line) == "--" && (len(cleaned) == 0 || strings.TrimSpace(cleaned[len(cleaned)-1]) == "--") {
				continue
			}
			cleaned = append(cleaned, line)
		}
		for len(cleaned) > 0 && strings.TrimSpace(cleaned[len(cleaned)-1]) == "--" {
			cleaned = cleaned[:len(cleaned)-1]
		}
		bodyLines = cleaned

		body := strings.Join(bodyLines, "\n")
		framed = append(framed, FrameContent(body, meta))
	}

	return strings.Join(framed, "\n")
}

// visibleLen returns the visible character count of a string, stripping ANSI codes.
func visibleLen(s string) int {
	return utf8.RuneCountInString(StripANSI(s))
}

// ripgrepLineNum extracts the line number from a ripgrep output line (e.g. "5:text" or "3-text").
// Returns 0 if the line doesn't start with a number.
func ripgrepLineNum(line string) int {
	n := 0
	for _, c := range line {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else if c == ':' || c == '-' {
			return n
		} else {
			return 0
		}
	}
	return 0
}

// stripRipgrepPrefix removes the "N:" or "N-" line number prefix from a ripgrep line.
// Leaves non-numbered lines (like "--" separators) unchanged.
func stripRipgrepPrefix(line string) string {
	for i, c := range line {
		if c >= '0' && c <= '9' {
			continue
		}
		if i > 0 && (c == ':' || c == '-') {
			return line[i+1:]
		}
		return line
	}
	return line
}

// styleSearchTags styles each tag individually — bold cyan for tags matching
// the query, dim cyan for the rest.
func styleSearchTags(tags, query string) string {
	q := strings.ToLower(query)
	parts := strings.Split(tags, ", ")
	for i, tag := range parts {
		if strings.Contains(strings.ToLower(tag), q) {
			parts[i] = BoldCyan.Render(tag)
		} else {
			parts[i] = DimCyan.Render(tag)
		}
	}
	return strings.Join(parts, DimCyan.Render(", "))
}

// truncateToWidth clips a string (which may contain ANSI escapes) to maxWidth
// visible characters, appending "…" if truncated. Adds an ANSI reset to close
// any open style sequences.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth < 2 {
		return "…"
	}
	cutoff := maxWidth - 1 // leave room for ellipsis
	visible := 0
	i := 0
	for i < len(s) {
		// Skip ANSI escape sequences
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++ // include 'm'
			}
			i = j
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		visible++
		if visible > cutoff {
			return s[:i] + "…\x1b[0m"
		}
		i += size
	}
	return s
}
