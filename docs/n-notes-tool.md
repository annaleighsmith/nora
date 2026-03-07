# `n` — Personal Note-Taking Tool

## What is this

A self-hosted, terminal-first note-taking system to replace Obsidian. Notes are plain markdown files on an always-on Linux box, accessed via CLI (neovim) on desktop and a web UI on phone via Tailscale. Claude API handles AI formatting of messy input and semantic querying across notes. No subscription, no sync, no lock-in.

## Architecture

```
Phone browser  --> Tailscale --> linux-box:3000 (web UI)
Desktop        --> terminal  --> `n` CLI commands + neovim
                                    |
                              ~/notes/ (markdown files)
                                    |
                              Claude API (format + query)
                                    |
                              git auto-commit (backup + history)
```

Single Go binary — CLI tool + embedded web server. No external dependencies at runtime.

## Tech Stack

- **Language**: Go
- **Web UI** (planned): Go `net/http` + `html/template` + HTMX + Pico CSS + CodeMirror
- **Assets**: Embedded in binary via `go:embed`
- **AI**: Claude API (Haiku 4.5 for formatting)
- **Search**: ripgrep (`rg`)
- **Interactive selection**: fzf
- **Editor**: neovim (via `$EDITOR`)
- **Access**: Tailscale for phone/remote access
- **Backup**: git auto-commit + optional remote push (planned)

## Current Project Structure

```
n-notes/
├── main.go            # entrypoint
├── cmd/
│   ├── root.go        # CLI routing, shortcut flags (-l, -e, -a, -n)
│   ├── add.go         # `add` (AI-formatted capture) + `new` (editor capture)
│   ├── search.go      # `look` (print note) + `edit` (open in editor), loadNotesDir helper
│   ├── setup.go       # interactive config setup
│   └── version.go     # version command
├── ai/
│   └── ai.go          # Claude API client + formatting prompt
├── notes/
│   └── search.go      # ripgrep + fzf integration
├── config/
│   └── config.go      # notes dir, API key, editor, port
├── go.mod
└── go.sum
```

## CLI Commands

### Implemented

```bash
n -l [query]               # find a note via fzf, print it (alias: n look)
n -e [query]               # find a note via fzf, open in editor (alias: n edit)
n -a "messy brain dump"    # AI formats + saves (alias: n add)
n -a                       # interactive stdin mode, AI formats + saves
n -n                       # new note in editor, AI formats + saves (alias: n new)
n setup                    # interactive config (notes dir, API key)
n version                  # print version
n help                     # print help
```

**Design rule**: all shortcut flags (`-l`, `-e`, `-a`, `-n`) accept either:
- Quoted inline text: `n -a "packing list"`, `n -l "sleep"`
- Interactive mode (no args): `n -a` → stdin prompt, `n -l` → unfiltered fzf

### Planned

```bash
n ask "question"           # search notes, AI answers citing filenames
n s <query>                # ripgrep search with formatted output
n recent [count]           # list recently modified notes
n tags                     # list all tags
n tag <tagname>            # list notes with that tag
n todo "thing to do"       # append to today's note or todos.md
n serve                    # start web UI on :3000
```

## AI Integration

### Formatting (`n add` / `n new`)
- System prompt describes note conventions (frontmatter with title/date/tags, clean prose, bullet points over numbered lists)
- Prompts Claude to wrap entire response in a single markdown code block — `stripCodeFences` strips it cleanly
- User input is the raw brain dump (inline args or stdin)
- AI returns formatted markdown, tool previews it and saves on confirmation
- Uses Claude Haiku 4.5 for speed

### Querying (`n ask` — planned)
- Run ripgrep for keyword matches from the query
- Read top ~5-10 matching files
- Send to Claude: "Based on these notes, answer: <question>. Cite note filenames."
- Print/stream the response

## Search Strategy

- **Primary**: ripgrep — fast, handles everything for a personal-sized vault
- **Interactive selection**: fzf for CLI fuzzy matching
- **No embeddings/vector DB to start** — add later only if grep stops being sufficient

## Note Storage

- **Location**: `~/notes/` (configurable via `n setup`)
- **Format**: plain markdown, YAML frontmatter (title, date, tags) added by AI formatting
- **Naming**: `YYYY-MM-DD-slug.md` — slug auto-generated from AI-assigned title
- **No folder hierarchy** — flat directory, tags for organization
- **Wiki links**: support `[[note-name]]` in web UI (planned)

## Web UI (Planned)

Mobile-first layout — primarily the phone interface via Tailscale.

### Pages
- **Home** (`/`): quick capture box, search/ask input, recent notes list
- **Note view** (`/note/:name`): rendered markdown, edit button
- **Note edit** (`/note/:name/edit`): CodeMirror editor, save via HTMX
- **Search**: HTMX partial, updates as you type (300ms debounce)
- **Ask**: streams AI response back (SSE or chunked response)

### Mobile layout
```
┌──────────────────────┐
│  n              + new │
├──────────────────────┤
│ search or ask...      │
├──────────────────────┤
│ Quick capture:        │
│ ┌──────────────────┐ │
│ │ type anything... │ │
│ └──────────────────┘ │
│           [save]      │
├──────────────────────┤
│ Today                 │
│  jake-api-discussion  │
│  daily                │
│ Yesterday             │
│  homepage-redesign    │
│  ideas                │
└──────────────────────┘
```

## Backup / Version History (Planned)

- Git auto-commit via file watcher goroutine in `n serve` or periodic cron
- Auto-generated commit messages
- Optional push to private remote for offsite backup
- Free version history: `git log notes/filename.md`

## What this replaces

| Obsidian | `n` |
|---|---|
| Desktop editor | neovim |
| Mobile app | Web UI via Tailscale |
| Sync ($8/mo) | Nothing — single machine |
| Search | `rg` + Claude API |
| Plugins | Your own code |
| AI features (paid) | Claude API (~$2/mo) |
| Vendor lock-in | Markdown files forever |

## Build Phases

### Phase 1: Core CLI ✅
- Project scaffolding, go module, config loading
- `notes/search.go` — ripgrep + fzf integration
- `n look`, `n edit` — find notes via fzf, print or open in editor
- Shortcut flags: `-l`, `-e`, `-a`, `-n` on root command with inline arg support

### Phase 2: AI Layer (in progress)
- Claude API client (Haiku 4.5)
- `n add` — AI formatting + save (inline args or stdin) ✅
- `n new` — editor capture + AI formatting + save ✅
- `n ask` — search + AI query (planned)

### Phase 3: Web UI
- `n serve` — HTTP server with embedded assets
- Home page with note list, search, capture
- Note view (rendered markdown) + edit (CodeMirror)
- HTMX search + ask with streaming

### Phase 4: Polish
- Git auto-commit
- Tag parsing and browsing
- `n todo` command
- Wiki link support
- Mobile CSS tuning

## Dependencies (Go modules)

- `github.com/spf13/cobra` — CLI framework
- `github.com/anthropics/anthropic-sdk-go` — Claude API
- `github.com/yuin/goldmark` — markdown to HTML (for web UI)
- System tools: `rg`, `fzf`, `git`
