package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ClaudeCodeProvider shells out to the `claude` CLI for completions.
// No API key needed — Claude Code handles its own auth and usage tracking.
type ClaudeCodeProvider struct{}

// claudeResponse is the JSON structure returned by `claude --output-format json`.
type claudeResponse struct {
	Result string `json:"result"`
}

func (p *ClaudeCodeProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	args := []string{
		"-p", req.UserMessage,
		"--output-format", "json",
		"--tools", "",
	}
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)

	// Strip ANTHROPIC_API_KEY so claude uses subscription auth, not the API key.
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "ANTHROPIC_API_KEY=") {
			cmd.Env = append(cmd.Env, env)
		}
	}

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return CompletionResponse{}, fmt.Errorf("claude CLI error: %s", string(exitErr.Stderr))
		}
		return CompletionResponse{}, fmt.Errorf("claude CLI error: %w", err)
	}

	var resp claudeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return CompletionResponse{}, fmt.Errorf("could not parse claude CLI output: %w", err)
	}

	if resp.Result == "" {
		return CompletionResponse{}, fmt.Errorf("empty result from claude CLI")
	}

	return CompletionResponse{
		Text:  resp.Result,
		Model: req.Model,
	}, nil
}
