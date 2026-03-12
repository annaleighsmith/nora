package ai

import (
	"context"
	"fmt"
)

// CompletionRequest is a provider-agnostic one-shot completion request.
type CompletionRequest struct {
	SystemPrompt string
	UserMessage  string
	MaxTokens    int
	Model        string // config alias like "haiku" — each provider resolves it
}

// CompletionResponse holds the result of a completion. Token fields are
// zero when the provider doesn't report them (e.g. Claude Code).
type CompletionResponse struct {
	Text          string
	InputTokens   int64
	OutputTokens  int64
	CacheCreation int64
	CacheRead     int64
	Model         string // resolved model name for usage tracking
}

// Provider is the interface for AI backends.
type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// GetProvider returns a Provider for the given name.
// Valid names: "anthropic" (default), "claude-code".
func GetProvider(providerName string) (Provider, error) {
	switch providerName {
	case "", "anthropic":
		return &AnthropicProvider{}, nil
	case "claude-code":
		return &ClaudeCodeProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q — valid options: anthropic, claude-code", providerName)
	}
}
