package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/annaleighsmith/nora/config"
	"github.com/anthropics/anthropic-sdk-go"
)

// RequireAPIKey returns the ANTHROPIC_API_KEY or an error if unset.
func RequireAPIKey() (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}
	return key, nil
}

// StripAnthropicKey returns env without ANTHROPIC_API_KEY so claude CLI
// uses subscription auth instead of the API key.
func StripAnthropicKey(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// AnthropicProvider calls the Anthropic Messages API directly.
type AnthropicProvider struct{}

func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if _, err := RequireAPIKey(); err != nil {
		return CompletionResponse{}, err
	}

	client := anthropic.NewClient()
	model := anthropic.Model(config.ResolveModel(req.Model))

	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: int64(req.MaxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.UserMessage)),
		},
	}
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	resp, err := client.Messages.New(ctx, params)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("claude API error: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return CompletionResponse{}, fmt.Errorf("no text in API response")
	}

	return CompletionResponse{
		Text:          text,
		InputTokens:   resp.Usage.InputTokens,
		OutputTokens:  resp.Usage.OutputTokens,
		CacheCreation: resp.Usage.CacheCreationInputTokens,
		CacheRead:     resp.Usage.CacheReadInputTokens,
		Model:         string(model),
	}, nil
}
