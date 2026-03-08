package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"github.com/annaleighsmith/nora/config"
	"github.com/annaleighsmith/nora/utils"

	"github.com/anthropics/anthropic-sdk-go"
)


const DefaultAskPrompt = `You are a knowledge assistant with access to the user's personal notes.

You have these tools for discovering and reading notes:
- search_notes: keyword search (prefix with # for tag search)
- read_note: read a specific note (with optional offset/limit)
- list_tags: see all tags in use — great starting point for broad questions
- list_recent_notes: see recently modified notes — great for "what's new?" questions
- note_index: compact list of ALL notes with titles and tags — use when search isn't finding what you need

Strategy:
- For specific questions, start with search_notes using keywords
- For broad/vague questions, start with list_tags or note_index to orient yourself
- For time-based questions ("lately", "recently"), use list_recent_notes
- Prefix a query with # to search by tag (e.g. "#project")
- Use specific keywords, not full sentences
- Search multiple angles if the first search doesn't find enough

When using read_note:
- You have a hard read budget per question. Each read is capped so you can't spend it all on one file.
- Before reading a long file, make sure it's worth the cost. Use search context and file length to judge relevance first.
- Spread your reads across multiple relevant notes rather than going deep on one.

When answering:
- Use bullet points, never numbered lists
- Cite note filenames in your answer so the user can find the source
- Be concise and direct
- If you can't find relevant notes, say so honestly
- Synthesize across multiple notes when relevant
- Give a complete, final answer. Do not ask follow-up questions or offer to do more.
- Do not add conversational filler like "Let me search" before tool calls — just call the tool.`

const DefaultManagePrompt = `You are a vault management assistant with full read and write access to the user's notes.

Read tools (use freely):
- search_notes: keyword search (prefix with # for tag search)
- read_note: read a specific note (with optional offset/limit)
- list_tags: see all tags in use
- list_recent_notes: see recently modified notes
- note_index: compact list of ALL notes with titles and tags

Write tools (require user confirmation before executing):
- delete_notes: permanently delete notes
- archive_notes: move notes to .archive/ (recoverable)
- create_note: create a new note (AI formats with frontmatter)
- add_tag: add a tag to notes
- remove_tag: remove a tag from notes
- fix_frontmatter: fix broken frontmatter fences

Strategy:
- Investigate thoroughly before proposing write actions
- Use search, read, and index tools to find the right notes
- When proposing writes, be specific about which files and why
- For bulk operations (tagging, deleting), show the full list
- If the user declines an action, adjust your approach

When responding:
- Use bullet points, never numbered lists
- Cite note filenames so the user can verify
- Be concise and direct
- Do not add conversational filler before tool calls — just call the tool.`

const maxToolCalls = 10

// ConfirmResponse holds the user's response to a confirmation prompt.
type ConfirmResponse struct {
	Approved bool
	Feedback string // non-empty when the user typed a revision instead of confirming
}

// ConfirmFunc is called when a write tool needs user confirmation.
// Enter (empty) = approved. Typed text = feedback for the AI to re-evaluate.
type ConfirmFunc func(ToolResult) ConfirmResponse

// Session holds conversation state for multi-turn ask/manage sessions.
type Session struct {
	messages    []anthropic.MessageParam
	client      anthropic.Client
	prompt      string
	model       anthropic.Model
	lightModel  anthropic.Model
	notesDir    string
	readBudget  int
	tools       []anthropic.ToolUnionParam
	handlers    map[string]ToolHandler
	confirmFn   ConfirmFunc
}

// NewSession creates a new conversational ask session (read-only tools).
func NewSession(notesDir string) (*Session, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}

	promptTemplate, err := config.LoadPrompt("ask", DefaultAskPrompt)
	if err != nil {
		return nil, err
	}

	promptTemplate = injectIdentity(promptTemplate, cfg)

	tools, handlers := ReadOnlyTools()

	return &Session{
		client:     anthropic.NewClient(),
		prompt:     promptTemplate,
		model:      anthropic.Model(config.ResolveModel(cfg.Models.Heavy)),
		lightModel: anthropic.Model(config.ResolveModel(cfg.Models.Light)),
		notesDir:   notesDir,
		readBudget: cfg.Bot.AskReadBudget,
		tools:      tools,
		handlers:   handlers,
	}, nil
}

// NewManageSession creates a manage session with read + write tools.
func NewManageSession(notesDir string, confirmFn ConfirmFunc) (*Session, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}

	promptTemplate, err := config.LoadPrompt("manage", DefaultManagePrompt)
	if err != nil {
		return nil, err
	}

	promptTemplate = injectIdentity(promptTemplate, cfg)

	tools, handlers := AllTools()

	return &Session{
		client:     anthropic.NewClient(),
		prompt:     promptTemplate,
		model:      anthropic.Model(config.ResolveModel(cfg.Models.Heavy)),
		lightModel: anthropic.Model(config.ResolveModel(cfg.Models.Light)),
		notesDir:   notesDir,
		readBudget: cfg.Bot.AskReadBudget,
		tools:      tools,
		handlers:   handlers,
		confirmFn:  confirmFn,
	}, nil
}

func injectIdentity(prompt string, cfg config.Config) string {
	if cfg.Bot.Name != "" {
		identity := "\n\nYour name is " + cfg.Bot.Name + "."
		if cfg.Bot.Personality != "" {
			identity += " " + cfg.Bot.Personality
		}
		prompt += identity
	}

	memories := loadMemories()
	if memories != "" {
		prompt += "\n\nYour memories from previous sessions (use these to guide your searches):\n" + memories
	}

	return prompt
}

// PreloadFile injects a file's content into the conversation as context
// so the AI can answer questions about it without tool calls.
func (s *Session) PreloadFile(filename, content string) {
	msg := fmt.Sprintf("Here is the note %s:\n\n%s", filename, content)
	s.messages = append(s.messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock(msg)),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock(fmt.Sprintf("I've read %s. What would you like to know?", filename))),
	)
}

// Ask sends a question (or follow-up) and returns cited files.
// Conversation history is preserved between calls.
func (s *Session) Send(question string) ([]string, error) {
	budget := &ReadBudget{Total: s.readBudget, Used: 0}
	s.messages = append(s.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(question)))

	ctx := context.Background()

	for range maxToolCalls {
		stream := s.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     s.model,
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: s.prompt},
			},
			Tools:    s.tools,
			Messages: s.messages,
		})

		var assistantBlocks []anthropic.ContentBlockParamUnion
		var currentToolID string
		var currentToolName string
		var currentToolInput string
		var currentText string
		var currentBlockType string
		var fullAnswer string
		var stopReason anthropic.StopReason

		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "message_start":
				msg := event.AsMessageStart()
				u := msg.Message.Usage
				Usage.Add(string(s.model), u.InputTokens, 0, u.CacheCreationInputTokens, u.CacheReadInputTokens)

			case "message_delta":
				delta := event.AsMessageDelta()
				stopReason = delta.Delta.StopReason
				u := delta.Usage
				Usage.Add(string(s.model), 0, u.OutputTokens, u.CacheCreationInputTokens, u.CacheReadInputTokens)

			case "content_block_start":
				cb := event.ContentBlock
				currentBlockType = cb.Type
				switch cb.Type {
				case "text":
					currentText = ""
				case "tool_use":
					currentToolID = cb.ID
					currentToolName = cb.Name
					currentToolInput = ""
				}

			case "content_block_delta":
				switch event.Delta.Type {
				case "text_delta":
					currentText += event.Delta.Text
				case "input_json_delta":
					currentToolInput += event.Delta.PartialJSON
				}

			case "content_block_stop":
				switch currentBlockType {
				case "text":
					assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(currentText))
					fullAnswer += currentText
				case "tool_use":
					if currentToolInput == "" {
						currentToolInput = "{}"
					}
					assistantBlocks = append(assistantBlocks,
						anthropic.NewToolUseBlock(currentToolID, json.RawMessage(currentToolInput), currentToolName))
					currentToolName = ""
				}
				currentBlockType = ""
			}
		}

		if err := stream.Err(); err != nil {
			return nil, fmt.Errorf("stream error: %w", err)
		}

		if len(assistantBlocks) > 0 {
			s.messages = append(s.messages, anthropic.NewAssistantMessage(assistantBlocks...))
		}

		if stopReason != anthropic.StopReasonToolUse {
			fmt.Println(utils.Render(fullAnswer))
			return extractCitedFiles(fullAnswer, s.notesDir), nil
		}

		// Process tool calls via handler dispatch
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range assistantBlocks {
			if block.OfToolUse == nil {
				continue
			}

			toolID := block.OfToolUse.ID
			toolName := block.OfToolUse.Name
			rawInput := block.OfToolUse.Input.(json.RawMessage)

			handler, ok := s.handlers[toolName]
			if !ok {
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, fmt.Sprintf("unknown tool: %s", toolName), true))
				continue
			}

			result := handler(s.notesDir, rawInput, budget)

			if result.NeedsConfirm {
				if s.confirmFn == nil {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, "No confirmation handler — action skipped.", false))
					continue
				}
				resp := s.confirmFn(result)
				if resp.Approved {
					output, err := result.Execute()
					if err != nil {
						toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, fmt.Sprintf("error: %v", err), true))
					} else {
						toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, output, false))
					}
				} else if resp.Feedback != "" {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID,
						fmt.Sprintf("User wants changes: %s", resp.Feedback), false))
				} else {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, "User declined this action.", false))
				}
				continue
			}

			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, result.Content, result.IsError))
		}

		s.messages = append(s.messages, anthropic.NewUserMessage(toolResults...))
	}

	fmt.Println()
	return nil, nil
}

// extractCitedFiles finds .md filenames mentioned in the AI answer and
// resolves them to full paths in the notes dir.
var citedFileRe = regexp.MustCompile(`\b([\w.-]+\.md)\b`)

func extractCitedFiles(answer, notesDir string) []string {
	var files []string
	seen := make(map[string]bool)

	for _, match := range citedFileRe.FindAllStringSubmatch(answer, -1) {
		name := match[1]
		if seen[name] {
			continue
		}

		fullPath := filepath.Join(notesDir, name)
		if _, err := os.Stat(fullPath); err != nil {
			continue
		}

		seen[name] = true
		files = append(files, fullPath)
	}
	return files
}

func memoriesPath() string {
	return filepath.Join(config.Dir(), "memories.md")
}

func loadMemories() string {
	data, err := os.ReadFile(memoriesPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

const memoryPrompt = `Extract a concise memory about the USER'S PREFERENCES from this conversation if you can — how they like things organized, formatted, tagged, or managed. Only save things that reveal how the user works or what they prefer.

Do NOT save anything about the content of their notes (topics, people, projects, facts) unless user specifically asks you to remember something. Generally, just save workflow and preference patterns.`

const consolidatePrompt = `Here are saved memories from previous sessions. Consolidate them: remove duplicates, keep the best version of each topic. Return only bullet points (- prefix), one per line. Do not add new information — just clean up what's here.`

// userTurnCount returns the number of user messages in the conversation,
// excluding tool_result messages (which are user-role but not real user turns).
func (s *Session) userTurnCount() int {
	count := 0
	for _, msg := range s.messages {
		if msg.Role != "user" {
			continue
		}
		// Tool result messages have OfToolResult blocks; skip those.
		isToolResult := false
		for _, block := range msg.Content {
			if block.OfToolResult != nil {
				isToolResult = true
				break
			}
		}
		if !isToolResult {
			count++
		}
	}
	return count
}

const minUserTurnsForMemory = 3

// SaveMemories extracts learnings from the session and appends to memories file.
// Only runs when the conversation had enough user turns (3+) to potentially
// reveal preferences — a single Q&A rarely contains anything worth remembering.
func (s *Session) SaveMemories() {
	if s.userTurnCount() < minUserTurnsForMemory {
		return
	}

	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Saving memories...\033[0m\n")

	// Ask the light model to extract memories from the conversation
	memMessages := make([]anthropic.MessageParam, len(s.messages))
	copy(memMessages, s.messages)
	memMessages = append(memMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(memoryPrompt)))

	ctx := context.Background()
	resp, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     s.model,
		MaxTokens: 512,
		Messages:  memMessages,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[2mCould not save memories: %v\033[0m\n", err)
		return
	}

	Usage.Add(string(s.model), resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheCreationInputTokens, resp.Usage.CacheReadInputTokens)

	var newMemories string
	for _, block := range resp.Content {
		if block.Type == "text" {
			newMemories = strings.TrimSpace(block.Text)
			break
		}
	}

	if newMemories == "" || strings.ToLower(newMemories) == "none" {
		fmt.Fprintf(os.Stderr, "\033[2m[TOOL] No new memories to save.\033[0m\n")
		return
	}

	// Append new memory to existing ones
	existing := loadMemories()
	allMemories := existing
	if allMemories != "" {
		allMemories += "\n"
	}
	allMemories += newMemories

	// Consolidate with light model to dedupe
	consolidateMessages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(consolidatePrompt + "\n\n" + allMemories)),
	}
	consResp, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     s.lightModel,
		MaxTokens: 1024,
		Messages:  consolidateMessages,
	})
	if err != nil {
		// If consolidation fails, just save the raw append
		os.WriteFile(memoriesPath(), []byte(allMemories), 0644)
		fmt.Fprintf(os.Stderr, "\033[2m[TOOL] Saved memory (consolidation failed).\033[0m\n")
		return
	}

	Usage.Add(string(s.lightModel), consResp.Usage.InputTokens, consResp.Usage.OutputTokens, consResp.Usage.CacheCreationInputTokens, consResp.Usage.CacheReadInputTokens)

	var consolidated string
	for _, block := range consResp.Content {
		if block.Type == "text" {
			consolidated = strings.TrimSpace(block.Text)
			break
		}
	}

	if consolidated == "" {
		consolidated = allMemories
	}

	os.WriteFile(memoriesPath(), []byte(consolidated+"\n"), 0644)

	// Count final memories
	count := 0
	for _, line := range strings.Split(consolidated, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-") || strings.HasPrefix(strings.TrimSpace(line), "•") {
			count++
		}
	}
	fmt.Fprintf(os.Stderr, "\033[2m[TOOL] %d memory(s) total.\033[0m\n", count)
}
