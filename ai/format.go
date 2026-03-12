package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/annaleighsmith/nora/config"
)

const DefaultFormatPrompt = `You are a note formatting assistant. Take the user's raw input and return a clean, well-structured markdown note.

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

// complete runs a one-shot completion through the configured provider,
// tracking usage and debug logs if the provider returns token data.
func complete(ctx context.Context, caller string, cfg config.Config, req CompletionRequest) (string, error) {
	provider, err := GetProvider(cfg.Models.Provider)
	if err != nil {
		return "", err
	}

	callStart := time.Now()
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	if resp.InputTokens > 0 || resp.OutputTokens > 0 {
		Usage.Add(resp.Model, resp.InputTokens, resp.OutputTokens, resp.CacheCreation, resp.CacheRead)
		DebugLog.Log(APILogEntry{
			Caller:        caller,
			Model:         resp.Model,
			LatencyMs:     time.Since(callStart).Milliseconds(),
			InputTokens:   resp.InputTokens,
			OutputTokens:  resp.OutputTokens,
			CacheCreation: resp.CacheCreation,
			CacheRead:     resp.CacheRead,
		})
	}

	return resp.Text, nil
}

// Format sends raw input through the configured AI provider for formatting.
// If originalFilename is non-empty, the AI will prefer it as the basis for
// the note title.
func Format(rawInput string, originalFilename string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}

	promptTemplate, err := config.LoadPrompt("format", DefaultFormatPrompt)
	if err != nil {
		return "", err
	}

	conventions := BuildConventions(cfg.Format)
	today := time.Now().Format(cfg.Format.DateFormat)
	filenameHint := strings.TrimSuffix(originalFilename, ".md")
	prompt := fmt.Sprintf(promptTemplate, conventions, today, filenameHint)

	return complete(context.Background(), "Format", cfg, CompletionRequest{
		SystemPrompt: prompt,
		UserMessage:  rawInput,
		MaxTokens:    2048,
		Model:        cfg.Models.Light,
	})
}

const DefaultFrontmatterPrompt = `You are a note metadata assistant. Given a filename and a preview of a markdown note, generate ONLY the YAML frontmatter block for it.

Conventions:
%s

Rules:
- The date should be: %s
- If the filename ("%s") is meaningful, use it as the basis for the title — clean it up (expand abbreviations, fix casing, replace separators with spaces) but preserve the intent
- If the filename is generic (e.g. "untitled", "note", "draft", timestamps only), derive the title from the content
- Return ONLY the frontmatter block (starting and ending with ---), nothing else`

const frontmatterPreviewLines = 20

// GenerateFrontmatter sends a snippet of the note (filename + first N lines)
// to the configured AI provider and gets back just the YAML frontmatter block.
// The original note content is preserved untouched.
func GenerateFrontmatter(fullContent string, originalFilename string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}

	promptTemplate, err := config.LoadPrompt("frontmatter", DefaultFrontmatterPrompt)
	if err != nil {
		return "", err
	}

	conventions := BuildConventions(cfg.Format)
	today := time.Now().Format(cfg.Format.DateFormat)
	filenameHint := strings.TrimSuffix(originalFilename, ".md")
	prompt := fmt.Sprintf(promptTemplate, conventions, today, filenameHint)

	preview := buildPreview(fullContent, originalFilename)

	return complete(context.Background(), "GenerateFrontmatter", cfg, CompletionRequest{
		SystemPrompt: prompt,
		UserMessage:  preview,
		MaxTokens:    512,
		Model:        cfg.Models.Light,
	})
}

// QuickQuery sends a one-shot prompt to the light model and returns the text response.
func QuickQuery(prompt string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}

	return complete(context.Background(), "QuickQuery", cfg, CompletionRequest{
		UserMessage: prompt,
		MaxTokens:   1024,
		Model:       cfg.Models.Light,
	})
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
