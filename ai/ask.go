package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"n-notes/config"
	"n-notes/notes"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultAskPrompt = `You are a knowledge assistant with access to the user's personal notes.

When the user asks a question, use the search_notes tool to find relevant notes. You can search multiple times with different queries to find all relevant information.

- Prefix a query with # to search by tag (e.g. "#project")
- Use specific keywords, not full sentences
- Search multiple angles if the first search doesn't find enough

When using read_note:
- Search results show each file's total line count. Use this to budget your reads.
- Try to stay under 500 total lines read across all read_note calls.
- For long files (100+ lines), use offset/limit to read only the relevant section rather than the whole file.
- You often don't need to read full files — the search context is usually enough.

When answering:
- Use bullet points, never numbered lists
- Cite note filenames in your answer so the user can find the source
- Be concise and direct
- If you can't find relevant notes, say so honestly
- Synthesize across multiple notes when relevant
- Give a complete, final answer. Do not ask follow-up questions or offer to do more.
- Do not add conversational filler like "Let me search" before tool calls — just call the tool.`

const maxToolCalls = 10

var searchNotesTool = anthropic.ToolUnionParam{
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
}

var readNoteTool = anthropic.ToolUnionParam{
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
}

// AskSession holds conversation state for multi-turn ask sessions.
type AskSession struct {
	messages   []anthropic.MessageParam
	client     anthropic.Client
	prompt     string
	model      anthropic.Model
	lightModel anthropic.Model
	notesDir   string
}

// NewAskSession creates a new conversational ask session.
func NewAskSession(notesDir string) (*AskSession, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}

	promptTemplate, err := config.LoadPrompt("ask", defaultAskPrompt)
	if err != nil {
		return nil, err
	}

	// Load existing memories into the system prompt
	memories := loadMemories()
	if memories != "" {
		promptTemplate += "\n\nYour memories from previous sessions (use these to guide your searches):\n" + memories
	}

	return &AskSession{
		client:     anthropic.NewClient(),
		prompt:     promptTemplate,
		model:      anthropic.Model(config.ResolveModel(cfg.Models.Heavy)),
		lightModel: anthropic.Model(config.ResolveModel(cfg.Models.Light)),
		notesDir:   notesDir,
	}, nil
}

// Ask sends a question (or follow-up) and returns cited files.
// Conversation history is preserved between calls.
func (s *AskSession) Ask(question string) ([]string, error) {
	s.messages = append(s.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(question)))

	ctx := context.Background()

	for range maxToolCalls {
		stream := s.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     s.model,
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: s.prompt},
			},
			Tools:    []anthropic.ToolUnionParam{searchNotesTool, readNoteTool},
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
				Usage.Add(string(s.model), msg.Message.Usage.InputTokens, 0)

			case "message_delta":
				delta := event.AsMessageDelta()
				stopReason = delta.Delta.StopReason
				Usage.Add(string(s.model), 0, delta.Usage.OutputTokens)

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
			fmt.Println(notes.Render(fullAnswer))
			return extractCitedFiles(fullAnswer, s.notesDir), nil
		}

		// Process tool calls
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range assistantBlocks {
			if block.OfToolUse == nil {
				continue
			}

			toolID := block.OfToolUse.ID
			rawInput := block.OfToolUse.Input.(json.RawMessage)

			switch block.OfToolUse.Name {
			case "search_notes":
				var input struct {
					Query string `json:"query"`
				}
				if err := json.Unmarshal(rawInput, &input); err != nil {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, fmt.Sprintf("invalid input: %v", err), true))
					continue
				}

				fmt.Fprintf(os.Stderr, "\033[2mSearching for %q...\033[0m\n", input.Query)

				result, err := notes.SearchNotes(s.notesDir, input.Query)
				if err != nil {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, fmt.Sprintf("search error: %v", err), true))
					continue
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, result, false))

			case "read_note":
				var input struct {
					Filename string `json:"filename"`
					Offset   int    `json:"offset"`
					Limit    int    `json:"limit"`
				}
				if err := json.Unmarshal(rawInput, &input); err != nil {
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, fmt.Sprintf("invalid input: %v", err), true))
					continue
				}

				fmt.Fprintf(os.Stderr, "\033[2mReading %s", input.Filename)
				if input.Offset > 0 || input.Limit > 0 {
					fmt.Fprintf(os.Stderr, " [offset:%d limit:%d]", input.Offset, input.Limit)
				}

				content, err := notes.ReadNote(s.notesDir, input.Filename, input.Offset, input.Limit)
				if err != nil {
					fmt.Fprintln(os.Stderr)
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, fmt.Sprintf("read error: %v", err), true))
					continue
				}
				contentLines := len(strings.Split(content, "\n"))
				fmt.Fprintf(os.Stderr, " (%d lines)\033[0m\n", contentLines)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolID, content, false))
			}
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

const memoryPrompt = `Review the conversation above. Extract only the most essential memory that would help answer future questions faster — one concise bullet point.

Good memories: "2026-02-20-valentine.md is a love letter to Keith, deeply personal and romantic"
Bad memories: restating details from the note, or things obvious from the filename/tags.

Return a single bullet point. If you learned nothing new or useful, return "none".
Do not repeat memories that already exist.`

// SaveMemories extracts learnings from the session and appends to memories file.
func (s *AskSession) SaveMemories() {
	if len(s.messages) < 2 {
		return
	}

	fmt.Fprintf(os.Stderr, "\033[2mSaving memories...\033[0m\n")

	// Ask the light model to extract memories from the conversation
	memMessages := make([]anthropic.MessageParam, len(s.messages))
	copy(memMessages, s.messages)
	memMessages = append(memMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(memoryPrompt)))

	ctx := context.Background()
	resp, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     s.lightModel,
		MaxTokens: 512,
		Messages:  memMessages,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[2mCould not save memories: %v\033[0m\n", err)
		return
	}

	Usage.Add(string(s.lightModel), resp.Usage.InputTokens, resp.Usage.OutputTokens)

	var newMemories string
	for _, block := range resp.Content {
		if block.Type == "text" {
			newMemories = strings.TrimSpace(block.Text)
			break
		}
	}

	if newMemories == "" || strings.ToLower(newMemories) == "none" {
		fmt.Fprintf(os.Stderr, "\033[2mNo new memories to save.\033[0m\n")
		return
	}

	// Count memories (bullet points)
	count := 0
	for _, line := range strings.Split(newMemories, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-") || strings.HasPrefix(strings.TrimSpace(line), "•") {
			count++
		}
	}

	// Append to memories file
	f, err := os.OpenFile(memoriesPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "\n%s\n", newMemories)

	fmt.Fprintf(os.Stderr, "\033[2mSaved %d new memory(s).\033[0m\n", count)
}
