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
	if !strings.HasPrefix(content, "---") {
		return false
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return false
	}
	fm := strings.ToLower(content[3 : 3+end])
	tag = strings.ToLower(tag)

	// Check inline format: tags: [foo, bar, baz]
	for _, line := range strings.Split(fm, "\n") {
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

// AddTag adds a tag to a note's frontmatter.
func AddTag(dir, name, tag string) error {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return fmt.Errorf("%s has no frontmatter", name)
	}

	end := strings.Index(content[3:], "---")
	if end == -1 {
		return fmt.Errorf("%s has malformed frontmatter", name)
	}

	fm := content[3 : 3+end]
	body := content[3+end:]

	// Find the tags line and append
	lines := strings.Split(fm, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "tags:") {
			continue
		}
		found = true
		after := strings.TrimPrefix(trimmed, "tags:")
		after = strings.TrimSpace(after)

		// Inline format: tags: [foo, bar]
		if strings.HasPrefix(after, "[") && strings.HasSuffix(after, "]") {
			inner := strings.TrimSuffix(strings.TrimPrefix(after, "["), "]")
			lines[i] = fmt.Sprintf("tags: [%s, %s]", inner, tag)
		} else if after != "" {
			// tags: foo, bar
			lines[i] = fmt.Sprintf("tags: %s, %s", after, tag)
		} else {
			// tags: (empty, add inline)
			lines[i] = fmt.Sprintf("tags: [%s]", tag)
		}
		break
	}

	if !found {
		// No tags line — add one before the closing ---
		lines = append(lines, fmt.Sprintf("tags: [%s]", tag))
	}

	newContent := "---" + strings.Join(lines, "\n") + body
	return os.WriteFile(path, []byte(newContent), 0644)
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
