package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"n-notes/config"
	"n-notes/utils"

	"github.com/anthropics/anthropic-sdk-go"
)

// ToolResult is returned by tool handlers. If NeedsConfirm is true, the
// caller must display Proposal and call Execute only after user confirmation.
type ToolResult struct {
	Content      string
	IsError      bool
	NeedsConfirm bool
	Proposal     string
	Execute      func() (string, error)
}

// ReadBudget tracks how many lines of note content have been read.
type ReadBudget struct {
	Total int
	Used  int
}

// ToolHandler processes a single tool call and returns a result.
type ToolHandler func(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult

// --- Tool definitions ---

var toolDefs = map[string]anthropic.ToolUnionParam{
	"search_notes": {
		OfTool: &anthropic.ToolParam{
			Name:        "search_notes",
			Description: anthropic.String("Search the user's notes using ripgrep. Returns matching filenames and context lines. Prefix query with # to search by tag."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search term. Use specific keywords, not full sentences. Prefix with # for tag search.",
					},
				},
				Required: []string{"query"},
			},
		},
	},
	"read_note": {
		OfTool: &anthropic.ToolParam{
			Name:        "read_note",
			Description: anthropic.String("Read content of a note by filename. Search results include total line counts per file — use that to decide whether to read the full file or a specific range. Omit offset/limit to read the entire note."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"filename": map[string]any{
						"type":        "string",
						"description": "The note filename (e.g. 2025-10-23-diet.md)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Start reading from this line number (0-based). Omit to start from the beginning.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of lines to return. Omit to read the entire note.",
					},
				},
				Required: []string{"filename"},
			},
		},
	},
	"list_tags": {
		OfTool: &anthropic.ToolParam{
			Name:        "list_tags",
			Description: anthropic.String("List all tags in use across the user's notes, with counts. No parameters needed. Use this to discover what topics and categories exist before searching."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{},
			},
		},
	},
	"list_recent_notes": {
		OfTool: &anthropic.ToolParam{
			Name:        "list_recent_notes",
			Description: anthropic.String("List the most recently modified notes. Use this for vague or time-based questions like 'what have I been writing about?' or 'what's new?'"),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"count": map[string]any{
						"type":        "integer",
						"description": "Number of recent notes to return (default 20, max 50).",
					},
				},
			},
		},
	},
	"note_index": {
		OfTool: &anthropic.ToolParam{
			Name:        "note_index",
			Description: anthropic.String("Get a compact index of ALL notes: filename, title, and tags. No content. Use this to get an overview of the entire vault or find notes when keyword search isn't working."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{},
			},
		},
	},
	"delete_notes": {
		OfTool: &anthropic.ToolParam{
			Name:        "delete_notes",
			Description: anthropic.String("Delete one or more notes. Requires user confirmation before executing. Use after investigating which notes should be deleted."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"files": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of note filenames to delete (e.g. [\"old-note.md\", \"scratch.md\"])",
					},
				},
				Required: []string{"files"},
			},
		},
	},
	"archive_notes": {
		OfTool: &anthropic.ToolParam{
			Name:        "archive_notes",
			Description: anthropic.String("Move one or more notes to the .archive/ directory. Requires user confirmation. Safer than deletion — notes can be recovered."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"files": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of note filenames to archive",
					},
				},
				Required: []string{"files"},
			},
		},
	},
	"create_note": {
		OfTool: &anthropic.ToolParam{
			Name:        "create_note",
			Description: anthropic.String("Create a new note. Provide the raw content and the AI will format it with proper frontmatter and save it. Requires user confirmation."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "A short descriptive title for the note",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The note content in markdown (without frontmatter — it will be generated)",
					},
					"tags": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Tags for the note (e.g. [\"garden\", \"planning\"])",
					},
				},
				Required: []string{"title", "content"},
			},
		},
	},
	"add_tag": {
		OfTool: &anthropic.ToolParam{
			Name:        "add_tag",
			Description: anthropic.String("Add a tag to one or more notes. Requires user confirmation."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"files": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of note filenames to tag",
					},
					"tag": map[string]any{
						"type":        "string",
						"description": "The tag to add (lowercase, no spaces)",
					},
				},
				Required: []string{"files", "tag"},
			},
		},
	},
	"remove_tag": {
		OfTool: &anthropic.ToolParam{
			Name:        "remove_tag",
			Description: anthropic.String("Remove a tag from one or more notes. Requires user confirmation."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"files": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of note filenames to remove the tag from",
					},
					"tag": map[string]any{
						"type":        "string",
						"description": "The tag to remove",
					},
				},
				Required: []string{"files", "tag"},
			},
		},
	},
	"fix_frontmatter": {
		OfTool: &anthropic.ToolParam{
			Name:        "fix_frontmatter",
			Description: anthropic.String("Fix broken frontmatter fences on one or more notes. Adds proper --- delimiters. Requires user confirmation."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"files": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of note filenames to fix. Use note_index or search to find candidates first.",
					},
				},
				Required: []string{"files"},
			},
		},
	},
}

// --- Tool handlers ---

var readToolNames = []string{"search_notes", "read_note", "list_tags", "list_recent_notes", "note_index"}
var writeToolNames = []string{"delete_notes", "archive_notes", "create_note", "add_tag", "remove_tag", "fix_frontmatter"}

var toolHandlers = map[string]ToolHandler{
	"search_notes":     handleSearchNotes,
	"read_note":        handleReadNote,
	"list_tags":        handleListTags,
	"list_recent_notes": handleListRecentNotes,
	"note_index":       handleNoteIndex,
	"delete_notes":     handleDeleteNotes,
	"archive_notes":    handleArchiveNotes,
	"create_note":      handleCreateNote,
	"add_tag":          handleAddTag,
	"remove_tag":       handleRemoveTag,
	"fix_frontmatter":  handleFixFrontmatter,
}

// ReadOnlyTools returns tool definitions and handlers for read-only sessions (ask).
func ReadOnlyTools() ([]anthropic.ToolUnionParam, map[string]ToolHandler) {
	var defs []anthropic.ToolUnionParam
	handlers := make(map[string]ToolHandler)
	for _, name := range readToolNames {
		defs = append(defs, toolDefs[name])
		handlers[name] = toolHandlers[name]
	}
	return defs, handlers
}

// AllTools returns tool definitions and handlers for manage sessions (read + write).
func AllTools() ([]anthropic.ToolUnionParam, map[string]ToolHandler) {
	var defs []anthropic.ToolUnionParam
	handlers := make(map[string]ToolHandler)
	allNames := append(readToolNames, writeToolNames...)
	for _, name := range allNames {
		defs = append(defs, toolDefs[name])
		handlers[name] = toolHandlers[name]
	}
	return defs, handlers
}

// --- Read tool handlers ---

func handleSearchNotes(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}

	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Searching for %q...\033[0m\n", input.Query)

	result, err := utils.SearchNotes(notesDir, input.Query)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("search error: %v", err), IsError: true}
	}
	return ToolResult{Content: result}
}

func handleReadNote(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Filename string `json:"filename"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}

	remaining := budget.Total - budget.Used
	if remaining <= 0 {
		fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Read budget exhausted (%d/%d lines)\033[0m\n", budget.Used, budget.Total)
		return ToolResult{Content: "Read budget exhausted. You have enough context — answer the question now. Do not attempt more reads."}
	}

	maxPerRead := budget.Total / 2
	if maxPerRead < 50 {
		maxPerRead = 50
	}
	cap := remaining - 1
	if cap < 1 {
		cap = 1
	}
	if maxPerRead > cap {
		maxPerRead = cap
	}
	if input.Limit <= 0 || input.Limit > maxPerRead {
		input.Limit = maxPerRead
	}

	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Reading %s", input.Filename)
	if input.Offset > 0 || input.Limit > 0 {
		fmt.Fprintf(os.Stderr, " [offset:%d limit:%d]", input.Offset, input.Limit)
	}

	content, err := utils.ReadNote(notesDir, input.Filename, input.Offset, input.Limit)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		return ToolResult{Content: fmt.Sprintf("read error: %v", err), IsError: true}
	}
	contentLines := len(strings.Split(content, "\n"))
	budget.Used += contentLines
	fmt.Fprintf(os.Stderr, " (%d lines, %d/%d budget)\033[0m\n", contentLines, budget.Used, budget.Total)
	return ToolResult{Content: content}
}

func handleListTags(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Listing tags...\033[0m\n")
	result, err := utils.ListTags(notesDir)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	return ToolResult{Content: result}
}

func handleListRecentNotes(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Count int `json:"count"`
	}
	json.Unmarshal(rawInput, &input)
	if input.Count <= 0 {
		input.Count = 20
	}
	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Listing %d recent notes...\033[0m\n", input.Count)
	result, err := utils.ListRecentNotes(notesDir, input.Count)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	return ToolResult{Content: result}
}

func handleNoteIndex(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Building note index...\033[0m\n")
	result, err := utils.NoteIndex(notesDir)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	return ToolResult{Content: result}
}

// --- Write tool handlers ---

func handleDeleteNotes(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}
	if len(input.Files) == 0 {
		return ToolResult{Content: "no files specified", IsError: true}
	}

	// Validate all files exist
	for _, f := range input.Files {
		path := filepath.Join(notesDir, f)
		if _, err := os.Stat(path); err != nil {
			return ToolResult{Content: fmt.Sprintf("file not found: %s", f), IsError: true}
		}
	}

	var proposal strings.Builder
	fmt.Fprintf(&proposal, "Delete %d note(s)?\n", len(input.Files))
	for _, f := range input.Files {
		fmt.Fprintf(&proposal, "  %s\n", f)
	}

	return ToolResult{
		NeedsConfirm: true,
		Proposal:     proposal.String(),
		Execute: func() (string, error) {
			for _, f := range input.Files {
				if err := os.Remove(filepath.Join(notesDir, f)); err != nil {
					return "", fmt.Errorf("could not delete %s: %w", f, err)
				}
			}
			return fmt.Sprintf("Deleted %d note(s).", len(input.Files)), nil
		},
	}
}

func handleArchiveNotes(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}
	if len(input.Files) == 0 {
		return ToolResult{Content: "no files specified", IsError: true}
	}

	for _, f := range input.Files {
		path := filepath.Join(notesDir, f)
		if _, err := os.Stat(path); err != nil {
			return ToolResult{Content: fmt.Sprintf("file not found: %s", f), IsError: true}
		}
	}

	var proposal strings.Builder
	fmt.Fprintf(&proposal, "Archive %d note(s) to .archive/?\n", len(input.Files))
	for _, f := range input.Files {
		fmt.Fprintf(&proposal, "  %s\n", f)
	}

	return ToolResult{
		NeedsConfirm: true,
		Proposal:     proposal.String(),
		Execute: func() (string, error) {
			archiveDir := filepath.Join(notesDir, ".archive")
			if err := os.MkdirAll(archiveDir, 0755); err != nil {
				return "", fmt.Errorf("could not create .archive directory: %w", err)
			}
			for _, f := range input.Files {
				src := filepath.Join(notesDir, f)
				dst := filepath.Join(archiveDir, f)
				if err := os.Rename(src, dst); err != nil {
					return "", fmt.Errorf("could not archive %s: %w", f, err)
				}
			}
			return fmt.Sprintf("Archived %d note(s).", len(input.Files)), nil
		},
	}
}

func handleCreateNote(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}
	if input.Title == "" || input.Content == "" {
		return ToolResult{Content: "title and content are required", IsError: true}
	}

	// Build raw input for the formatter
	var raw strings.Builder
	raw.WriteString("Title: " + input.Title + "\n\n")
	raw.WriteString(input.Content)
	if len(input.Tags) > 0 {
		raw.WriteString("\n\nTags: " + strings.Join(input.Tags, ", "))
	}

	// Format via light model
	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Formatting note...\033[0m\n")
	formatted, err := Format(raw.String(), "")
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("formatting error: %v", err), IsError: true}
	}
	formatted = stripCreateFences(formatted)

	// Generate filename from the formatted content
	filename := generateCreateFilename(formatted)
	fullPath := filepath.Join(notesDir, filename)

	var proposal strings.Builder
	fmt.Fprintf(&proposal, "Create note %s?\n\n%s", filename, formatted)

	return ToolResult{
		NeedsConfirm: true,
		Proposal:     proposal.String(),
		Execute: func() (string, error) {
			if err := os.WriteFile(fullPath, []byte(formatted+"\n"), 0644); err != nil {
				return "", fmt.Errorf("could not save note: %w", err)
			}
			return fmt.Sprintf("Created %s", filename), nil
		},
	}
}

func handleAddTag(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Files []string `json:"files"`
		Tag   string   `json:"tag"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}
	if len(input.Files) == 0 || input.Tag == "" {
		return ToolResult{Content: "files and tag are required", IsError: true}
	}

	var proposal strings.Builder
	fmt.Fprintf(&proposal, "Add tag %q to %d note(s)?\n", input.Tag, len(input.Files))
	for _, f := range input.Files {
		fmt.Fprintf(&proposal, "  %s\n", f)
	}

	return ToolResult{
		NeedsConfirm: true,
		Proposal:     proposal.String(),
		Execute: func() (string, error) {
			for _, f := range input.Files {
				if err := utils.AddTag(notesDir, f, input.Tag); err != nil {
					return "", fmt.Errorf("could not tag %s: %w", f, err)
				}
			}
			return fmt.Sprintf("Tagged %d note(s) with %q.", len(input.Files), input.Tag), nil
		},
	}
}

func handleRemoveTag(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Files []string `json:"files"`
		Tag   string   `json:"tag"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}
	if len(input.Files) == 0 || input.Tag == "" {
		return ToolResult{Content: "files and tag are required", IsError: true}
	}

	var proposal strings.Builder
	fmt.Fprintf(&proposal, "Remove tag %q from %d note(s)?\n", input.Tag, len(input.Files))
	for _, f := range input.Files {
		fmt.Fprintf(&proposal, "  %s\n", f)
	}

	return ToolResult{
		NeedsConfirm: true,
		Proposal:     proposal.String(),
		Execute: func() (string, error) {
			for _, f := range input.Files {
				if err := utils.RemoveTag(notesDir, f, input.Tag); err != nil {
					return "", fmt.Errorf("could not remove tag from %s: %w", f, err)
				}
			}
			return fmt.Sprintf("Removed tag %q from %d note(s).", input.Tag, len(input.Files)), nil
		},
	}
}

func handleFixFrontmatter(notesDir string, rawInput json.RawMessage, budget *ReadBudget) ToolResult {
	var input struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}
	}
	if len(input.Files) == 0 {
		return ToolResult{Content: "no files specified", IsError: true}
	}

	var proposal strings.Builder
	fmt.Fprintf(&proposal, "Fix frontmatter fences on %d note(s)?\n", len(input.Files))
	for _, f := range input.Files {
		fmt.Fprintf(&proposal, "  %s\n", f)
	}

	return ToolResult{
		NeedsConfirm: true,
		Proposal:     proposal.String(),
		Execute: func() (string, error) {
			fixed := 0
			for _, f := range input.Files {
				path := filepath.Join(notesDir, f)
				changed, err := utils.FixFrontmatterFences(path)
				if err != nil {
					return "", fmt.Errorf("could not fix %s: %w", f, err)
				}
				if changed {
					fixed++
				}
			}
			return fmt.Sprintf("Fixed frontmatter on %d note(s).", fixed), nil
		},
	}
}

// --- Helpers for create_note ---

func stripCreateFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i != -1 {
			s = s[i+1:]
		}
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimRight(s, "\n")
	}
	return s
}

func generateCreateFilename(content string) string {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	title := extractCreateTitle(content)
	date := extractCreateDate(content)
	if date == "" {
		date = time.Now().Format(cfg.Format.DateFormat)
	}
	slug := slugifyCreate(title, cfg.Format.SlugStyle)

	name := cfg.Format.Naming
	name = strings.ReplaceAll(name, "{date}", date)
	name = strings.ReplaceAll(name, "{slug}", slug)
	return name + ".md"
}

func extractCreateTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "title:") {
			title := strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			return strings.Trim(title, "\"'")
		}
	}
	return "note"
}

func extractCreateDate(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "date:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "date:"))
		}
	}
	return ""
}

func slugifyCreate(s string, style string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	sep := "-"
	if style == "snake" {
		sep = "_"
	}
	s = reg.ReplaceAllString(s, sep)
	s = strings.Trim(s, sep)
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, sep)
	}
	return s
}
