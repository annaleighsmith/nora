package ai

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"n-notes/config"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultFormatPrompt = `You are a note formatting assistant. Take the user's raw input and return a clean, well-structured markdown note.

Note conventions:
%s

Additional rules:
- The date should be: %s
- If an original filename is provided ("%s"), use it as the basis for the title unless it is generic (e.g. "untitled", "note", "draft", timestamps only). Clean it up (expand abbreviations, fix casing) but preserve the intent.
- If no original filename is provided, generate a concise, descriptive title from the content.
- Clean up grammar and structure, but preserve the original meaning and voice
- Do not add information that wasn't in the original input
- Return ONLY the formatted note, no explanations or commentary
- Wrap your entire response in a single markdown code block`

// BuildConventions generates a conventions string from format config.
func BuildConventions(fc config.FormatConfig) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("- Start with YAML frontmatter containing: %s",
		strings.Join(fc.Frontmatter, ", ")))

	lines = append(lines, "- Tags should be 1-4 relevant lowercase keywords as a YAML list, e.g. tags: [foo, bar, baz]")

	switch fc.ListStyle {
	case "numbered":
		lines = append(lines, "- Use numbered lists, not bullet points")
	default:
		lines = append(lines, "- Use bullet points, not numbered lists")
	}

	return strings.Join(lines, "\n")
}

// Format sends raw input through Claude for formatting. If originalFilename
// is non-empty, the AI will prefer it as the basis for the note title.
func Format(rawInput string, originalFilename string) (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}

	promptTemplate, err := config.LoadPrompt("format", defaultFormatPrompt)
	if err != nil {
		return "", err
	}

	client := anthropic.NewClient()

	conventions := BuildConventions(cfg.Format)
	today := time.Now().Format(cfg.Format.DateFormat)

	// Strip .md extension from filename hint for cleaner prompt context
	filenameHint := strings.TrimSuffix(originalFilename, ".md")
	prompt := fmt.Sprintf(promptTemplate, conventions, today, filenameHint)

	model := anthropic.Model(config.ResolveModel(cfg.Models.Light))

	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{
			{Text: prompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(rawInput)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude API error: %w", err)
	}

	Usage.Add(string(model), resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheCreationInputTokens, resp.Usage.CacheReadInputTokens)

	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text in API response")
}

const defaultFrontmatterPrompt = `You are a note metadata assistant. Given a filename and a preview of a markdown note, generate ONLY the YAML frontmatter block for it.

Conventions:
%s

Rules:
- The date should be: %s
- If the filename ("%s") is meaningful, use it as the basis for the title — clean it up (expand abbreviations, fix casing, replace separators with spaces) but preserve the intent
- If the filename is generic (e.g. "untitled", "note", "draft", timestamps only), derive the title from the content
- Return ONLY the frontmatter block (starting and ending with ---), nothing else`

const frontmatterPreviewLines = 20

// GenerateFrontmatter sends a snippet of the note (filename + first N lines)
// to Claude and gets back just the YAML frontmatter block. The original note
// content is preserved untouched.
func GenerateFrontmatter(fullContent string, originalFilename string) (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}

	promptTemplate, err := config.LoadPrompt("frontmatter", defaultFrontmatterPrompt)
	if err != nil {
		return "", err
	}

	client := anthropic.NewClient()

	conventions := BuildConventions(cfg.Format)
	today := time.Now().Format(cfg.Format.DateFormat)
	filenameHint := strings.TrimSuffix(originalFilename, ".md")
	prompt := fmt.Sprintf(promptTemplate, conventions, today, filenameHint)

	model := anthropic.Model(config.ResolveModel(cfg.Models.Light))

	// Build a truncated preview to send instead of the full content
	preview := buildPreview(fullContent, originalFilename)

	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: prompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(preview)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude API error: %w", err)
	}

	Usage.Add(string(model), resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheCreationInputTokens, resp.Usage.CacheReadInputTokens)

	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text in API response")
}

// QuickQuery sends a one-shot prompt to the light model and returns the text response.
func QuickQuery(prompt string) (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}

	client := anthropic.NewClient()
	model := anthropic.Model(config.ResolveModel(cfg.Models.Light))

	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude API error: %w", err)
	}

	Usage.Add(string(model), resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheCreationInputTokens, resp.Usage.CacheReadInputTokens)

	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("no text in API response")
}

func buildPreview(content string, filename string) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	preview := lines
	if totalLines > frontmatterPreviewLines {
		preview = lines[:frontmatterPreviewLines]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "filename: %s\n---\n", filename)
	b.WriteString(strings.Join(preview, "\n"))
	if totalLines > frontmatterPreviewLines {
		fmt.Fprintf(&b, "\n---\n(%d more lines)", totalLines-frontmatterPreviewLines)
	}
	return b.String()
}
