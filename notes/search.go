package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Pick opens fzf to select a note, optionally pre-filtered by query.
// Returns the selected file path, or empty string if cancelled.
func Pick(dir, query string, inline bool) (string, error) {
	fzfArgs := []string{
		"--preview", "cat {}",
		"--preview-window", "right:60%:wrap",
	}

	if inline {
		fzfArgs = append(fzfArgs, "--height=40%", "--layout=reverse")
	}

	if query != "" {
		fzfArgs = append(fzfArgs, "--query", query)
	}

	fzf := exec.Command("fzf", fzfArgs...)
	fzf.Stderr = os.Stderr

	if query != "" {
		rg := exec.Command("rg", "--files-with-matches", "--no-messages", query, dir)
		out, _ := rg.Output()
		if len(out) > 0 {
			fzf.Stdin = strings.NewReader(string(out))
		} else {
			files, _ := listNotes(dir)
			fzf.Stdin = strings.NewReader(strings.Join(files, "\n"))
		}
	} else {
		files, err := listNotes(dir)
		if err != nil {
			return "", err
		}
		fzf.Stdin = strings.NewReader(strings.Join(files, "\n"))
	}

	selected, err := fzf.Output()
	if err != nil {
		return "", nil
	}

	return strings.TrimSpace(string(selected)), nil
}

// Look picks a note with inline fzf and prints its content.
func Look(dir, query string) error {
	path, err := Pick(dir, query, true)
	if err != nil || path == "" {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", path, err)
	}

	fmt.Print(string(content))
	return nil
}

// Open picks a note with full-screen fzf and opens it in $EDITOR.
func Open(dir, query string) error {
	path, err := Pick(dir, query, false)
	if err != nil || path == "" {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func listNotes(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	return matches, nil
}
