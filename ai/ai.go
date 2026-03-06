package ai

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const formatPrompt = `You are a note formatting assistant. Take the user's raw input and return a clean, well-structured markdown note.

Rules:
- Start with YAML frontmatter containing: title, date, tags
- The title should be concise and descriptive
- The date should be: %s
- Choose 1-4 relevant tags as a YAML list
- Use bullet points, not numbered lists
- Clean up grammar and structure, but preserve the original meaning and voice
- Do not add information that wasn't in the original input
- Return ONLY the formatted note, no explanations or commentary
- Wrap your entire response in a single markdown code block`

func Format(rawInput string) (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	client := anthropic.NewClient()

	today := time.Now().Format("2006-01-02")
	prompt := fmt.Sprintf(formatPrompt, today)

	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
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

	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text in API response")
}
