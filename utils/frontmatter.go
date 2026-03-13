package utils

import "strings"

// Frontmatter holds parsed YAML frontmatter fields.
type Frontmatter struct {
	Title string
	Date  string
	Tags  []string
}

// ParseFrontmatter extracts title, date, and tags from YAML frontmatter.
// Returns the parsed fields and the body (content after frontmatter).
func ParseFrontmatter(content string) (Frontmatter, string) {
	block, body := splitFrontmatter(content)
	if block == "" {
		return Frontmatter{}, body
	}

	var fm Frontmatter
	inTags := false
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "title:") {
			fm.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "title:"))
			fm.Title = strings.Trim(fm.Title, "\"'")
		} else if strings.HasPrefix(trimmed, "date:") {
			fm.Date = strings.TrimSpace(strings.TrimPrefix(trimmed, "date:"))
			fm.Date = strings.Trim(fm.Date, "\"'")
		} else if strings.HasPrefix(trimmed, "tags:") {
			inTags = true
			after := strings.TrimSpace(strings.TrimPrefix(trimmed, "tags:"))
			if after != "" {
				after = strings.Trim(after, "[]")
				for _, t := range strings.Split(after, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						fm.Tags = append(fm.Tags, t)
					}
				}
				inTags = false
			}
		} else if inTags && strings.HasPrefix(trimmed, "-") {
			tag := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if tag != "" {
				fm.Tags = append(fm.Tags, tag)
			}
		} else if inTags && trimmed != "" {
			inTags = false
		}
	}

	return fm, body
}

// splitFrontmatter splits content into the frontmatter block (without --- delimiters)
// and the body. Returns empty block if no frontmatter is found.
func splitFrontmatter(content string) (block, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return "", content
	}
	block = content[3 : 3+end]
	body = strings.TrimLeft(content[3+end+3:], "\n")
	return block, body
}

// FrontmatterEndLine returns the 1-indexed line number of the closing "---",
// or 0 if there's no frontmatter.
func FrontmatterEndLine(content string) int {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 1 // 1-indexed to match ripgrep
		}
	}
	return 0
}

// HasTag checks if content has the given tag in its frontmatter (case-insensitive).
func HasTag(content, tag string) bool {
	fm, _ := ParseFrontmatter(content)
	tag = strings.ToLower(tag)
	for _, t := range fm.Tags {
		if strings.ToLower(t) == tag {
			return true
		}
	}
	return false
}
