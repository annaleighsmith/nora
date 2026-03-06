package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindUntagged finds notes that match a keyword but don't have it as a tag.
func FindUntagged(dir, tag string) ([]string, error) {
	// Find all files containing the keyword
	cmd := exec.Command("rg", "--no-messages", "--files-with-matches", "--ignore-case", tag, dir)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep error: %w", err)
	}

	var candidates []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		name := filepath.Base(line)
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if hasTag(filepath.Join(dir, name), tag) {
			continue
		}
		candidates = append(candidates, name)
	}
	return candidates, nil
}

// hasTag checks if a note already has a specific tag in its frontmatter.
func hasTag(path, tag string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	fm := extractFrontmatterBlock(content)
	if fm == "" {
		return false
	}
	tag = strings.ToLower(tag)

	for _, line := range strings.Split(strings.ToLower(fm), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "tags:") {
			continue
		}
		after := strings.TrimPrefix(trimmed, "tags:")
		after = strings.Trim(strings.TrimSpace(after), "[]")
		for _, t := range strings.Split(after, ",") {
			if strings.TrimSpace(t) == tag {
				return true
			}
		}
		return false
	}
	return false
}

// extractFrontmatterBlock returns the frontmatter content, handling both
// fenced (---) and unfenced formats.
func extractFrontmatterBlock(content string) string {
	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end != -1 {
			return content[3 : 3+end]
		}
	}
	// Unfenced: look for tags:/title:/date: at the start
	lines := strings.Split(content, "\n")
	var fmLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		if trimmed == "---" {
			continue
		}
		if strings.Contains(trimmed, ":") {
			fmLines = append(fmLines, line)
		} else {
			break
		}
	}
	if len(fmLines) > 0 {
		return strings.Join(fmLines, "\n")
	}
	return ""
}

// FixFrontmatterFences ensures a note has proper --- fences around its frontmatter.
// Returns true if the file was modified.
func FixFrontmatterFences(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	content := string(data)

	// Already properly fenced
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---"); end != -1 {
			return false, nil
		}
	}

	// Find unfenced frontmatter lines at the start
	lines := strings.Split(content, "\n")
	fmEnd := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			continue
		}
		if trimmed == "" {
			fmEnd = i
			break
		}
		// Frontmatter lines have key: value format
		if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "#") {
			fmEnd = i + 1
		} else {
			break
		}
	}

	if fmEnd == 0 {
		return false, nil // no frontmatter detected
	}

	// Collect frontmatter lines (skip any stray --- lines)
	var fmLines []string
	for i := 0; i < fmEnd; i++ {
		if strings.TrimSpace(lines[i]) != "---" {
			fmLines = append(fmLines, lines[i])
		}
	}

	// Rebuild with proper fences
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, line := range fmLines {
		sb.WriteString(line + "\n")
	}
	sb.WriteString("---\n")

	// Append remaining content, skipping blank lines between fm and body
	remaining := lines[fmEnd:]
	started := false
	for _, line := range remaining {
		if !started && strings.TrimSpace(line) == "" {
			continue
		}
		started = true
		sb.WriteString(line + "\n")
	}

	result := strings.TrimRight(sb.String(), "\n") + "\n"
	if result == content {
		return false, nil
	}

	return true, os.WriteFile(path, []byte(result), 0644)
}

// FindBrokenFrontmatter returns filenames of notes with unfenced or missing frontmatter.
func FindBrokenFrontmatter(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var broken []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)

		// Properly fenced
		if strings.HasPrefix(content, "---\n") {
			if end := strings.Index(content[4:], "\n---"); end != -1 {
				continue
			}
		}

		// Check if there's unfenced frontmatter
		fm := extractFrontmatterBlock(content)
		if fm != "" {
			broken = append(broken, e.Name())
		}
	}
	return broken, nil
}

// AddTag adds a tag to a note's frontmatter.
func AddTag(dir, name, tag string) error {
	path := filepath.Join(dir, name)

	// Auto-fix fences before modifying
	FixFrontmatterFences(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find the tags line anywhere in the first section
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "tags:") {
			continue
		}
		found = true
		after := strings.TrimPrefix(trimmed, "tags:")
		after = strings.TrimSpace(after)

		if strings.HasPrefix(after, "[") && strings.HasSuffix(after, "]") {
			inner := strings.TrimSuffix(strings.TrimPrefix(after, "["), "]")
			lines[i] = fmt.Sprintf("tags: [%s, %s]", inner, tag)
		} else if after != "" {
			lines[i] = fmt.Sprintf("tags: %s, %s", after, tag)
		} else {
			lines[i] = fmt.Sprintf("tags: [%s]", tag)
		}
		break
	}

	if !found {
		return fmt.Errorf("%s has no tags line in frontmatter", name)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// FirstLines returns the first n lines of a note's content (after frontmatter).
func FirstLines(dir, name string, n int) string {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	content := string(data)
	// Skip frontmatter
	if strings.HasPrefix(content, "---") {
		if end := strings.Index(content[3:], "---"); end != -1 {
			content = content[3+end+3:]
		}
	}

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
