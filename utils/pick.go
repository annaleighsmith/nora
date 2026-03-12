package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"golang.org/x/term"
)

// Pick opens fzf to select a note with fuzzy matching across filenames and content.
// Returns the selected file path, or empty string if cancelled.
func Pick(dir, query string, inline bool) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine executable path: %w", err)
	}

	// For #tag queries, use ripgrep to scope to matching files first
	if query != "" && strings.HasPrefix(query, "#") {
		return pickByTag(dir, query, inline, self)
	}

	// Build "path\tfilename — title and preview" lines for fuzzy matching
	files, err := listNotes(dir)
	if err != nil {
		return "", err
	}

	var lines []string
	for _, f := range files {
		name := filepath.Base(f)
		data, _ := os.ReadFile(f)
		title, _ := extractFrontmatterMeta(string(data))
		preview := FirstLines(dir, name, 2)
		preview = strings.ReplaceAll(preview, "\n", " ")

		display := name
		if title != "" && title != name {
			display = name + " — " + title
		}
		if preview != "" {
			display += " | " + preview
		}
		// Format: full_path\tdisplay_text — fzf shows field 2+, returns field 1
		lines = append(lines, f+"\t"+display)
	}

	fzfArgs := []string{
		"--delimiter", "\t",
		"--with-nth", "2",
		"--preview", fmt.Sprintf("%s _preview {1}", self),
		"--preview-window", "right:60%:wrap",
	}

	if inline {
		fzfArgs = append(fzfArgs, "--height=40%", "--layout=reverse")
	}

	if query != "" {
		fzfArgs = append(fzfArgs, "--query", query)
	}

	fzf := exec.Command("fzf", fzfArgs...)
	fzf.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	fzf.Stderr = os.Stderr

	selected, err := fzf.Output()
	if err != nil {
		return "", nil
	}

	// Extract the path (first field before tab)
	result := strings.TrimSpace(string(selected))
	if i := strings.Index(result, "\t"); i != -1 {
		result = result[:i]
	}
	return result, nil
}

// pickByTag handles #tag queries with ripgrep scoping.
func pickByTag(dir, query string, inline bool, self string) (string, error) {
	tag := strings.TrimPrefix(query, "#")
	rg := exec.Command("rg", "--files-with-matches", "--no-messages",
		"tags:.*"+tag+"|^\\s*-\\s*"+tag, dir)
	out, _ := rg.Output()

	var paths []string
	if len(out) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				paths = append(paths, line)
			}
		}
	}
	if len(paths) == 0 {
		paths, _ = listNotes(dir)
	}

	return PickFrom(paths, inline)
}

// PickFrom opens fzf seeded with a specific list of file paths.
// Returns the selected file path, or empty string if cancelled.
func PickFrom(paths []string, inline bool) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine executable path: %w", err)
	}

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

// PickMultiFrom opens fzf in multi-select mode seeded with a specific list of file paths.
// Returns selected file paths, or nil if cancelled.
func PickMultiFrom(paths []string, inline bool) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not determine executable path: %w", err)
	}

	fzfArgs := []string{
		"--multi",
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

	output, err := fzf.Output()
	if err != nil {
		return nil, nil
	}

	var selected []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			selected = append(selected, line)
		}
	}
	return selected, nil
}

// Show picks a note with inline fzf and prints its content.
func Show(dir, query string) error {
	path, err := Pick(dir, query, true)
	if err != nil || path == "" {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", path, err)
	}

	Dim.Println(filepath.Base(path))
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
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(termStyle()),
		glamour.WithWordWrap(width),
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
