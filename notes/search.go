package notes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

// Pick opens fzf to select a note, with live ripgrep content search.
// Returns the selected file path, or empty string if cancelled.
func Pick(dir, query string, inline bool) (string, error) {
	// Shell command for fzf reload: #tag searches frontmatter tags only
	rgCmd := fmt.Sprintf(
		`q={q}; if [ "${q#\#}" != "$q" ]; then tag="${q#\#}"; rg --files-with-matches --no-messages "tags:.*$tag|^\s*-\s*$tag" %s; else rg --files-with-matches --no-messages "$q" %s || ls %s/*.md; fi`,
		dir, dir, dir)

	self, _ := os.Executable()

	fzfArgs := []string{
		"--disabled",
		"--delimiter", "/",
		"--with-nth", "-1",
		"--bind", "change:reload:" + rgCmd,
		"--preview", fmt.Sprintf("%s _preview {}", self),
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

	// Seed initial list: #tag scopes to frontmatter, otherwise full content search
	if query != "" && strings.HasPrefix(query, "#") {
		tag := strings.TrimPrefix(query, "#")
		rg := exec.Command("rg", "--files-with-matches", "--no-messages",
			"tags:.*"+tag+"|^\\s*-\\s*"+tag, dir)
		out, _ := rg.Output()
		if len(out) > 0 {
			fzf.Stdin = strings.NewReader(string(out))
		} else {
			files, _ := listNotes(dir)
			fzf.Stdin = strings.NewReader(strings.Join(files, "\n"))
		}
	} else if query != "" {
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

// PickFrom opens fzf seeded with a specific list of file paths.
// Returns the selected file path, or empty string if cancelled.
func PickFrom(paths []string, inline bool) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	self, _ := os.Executable()

	fzfArgs := []string{
		"--delimiter", "/",
		"--with-nth", "-1",
		"--preview", fmt.Sprintf("%s _preview {}", self),
		"--preview-window", "right:60%:wrap",
	}

	if inline {
		fzfArgs = append(fzfArgs, "--height=40%", "--layout=reverse")
	}

	fzf := exec.Command("fzf", fzfArgs...)
	fzf.Stdin = strings.NewReader(strings.Join(paths, "\n"))
	fzf.Stderr = os.Stderr

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

	fmt.Printf("\033[2m%s\033[0m\n", filepath.Base(path))
	fmt.Println(Render(string(content)))
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

// Render formats markdown content for terminal display using glamour.
func Render(content string) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(termStyle()),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return content
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

// termStyle returns a glamour style that uses ANSI colors (inherits terminal theme).
func termStyle() ansi.StyleConfig {
	bTrue := true
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			Margin: uintPtr(0),
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  &bTrue,
				Color: stringPtr("4"),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  &bTrue,
				Color: stringPtr("5"),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  &bTrue,
				Color: stringPtr("4"),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  &bTrue,
				Color: stringPtr("6"),
			},
		},
		Link: ansi.StylePrimitive{
			Color: stringPtr("4"),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr("6"),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("3"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(1),
			},
		},
		Emph: ansi.StylePrimitive{
			Color: stringPtr("3"),
		},
		Strong: ansi.StylePrimitive{
			Bold:  &bTrue,
			Color: stringPtr("1"),
		},
		List: ansi.StyleList{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(0),
			},
		},
	}
}

func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }

func listNotes(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	return matches, nil
}
