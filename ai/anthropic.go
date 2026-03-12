package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/annaleighsmith/nora/config"
	"github.com/anthropics/anthropic-sdk-go"
)

// AnthropicProvider calls the Anthropic Messages API directly.
type AnthropicProvider struct{}

func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return CompletionResponse{}, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
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
