# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`n` is a self-hosted, terminal-first note-taking tool written in Go. Single binary that provides both a CLI (neovim-based editing, ripgrep search, fzf selection) and an embedded web server (mobile UI via Tailscale). Claude API handles AI formatting of messy input and semantic querying across notes. Notes are plain markdown in a flat directory (`~/notes/`).

## Build & Run

```bash
go build -o n .           # build binary
go run .                  # run directly
go test ./...             # run all tests
go test ./notes/...       # run tests for a package
go test -run TestName ./pkg/  # run a single test
```

### System dependencies (must be on PATH)
- `rg` (ripgrep) — search
- `fzf` — interactive fuzzy selection in CLI
- `git` — auto-commit / version history
- `$EDITOR` (defaults to neovim) — note editing

## Architecture

```
main.go (entrypoint, CLI routing via cobra)
  cmd/       CLI command implementations
  ai/        Claude API client, formatting + querying prompts
  notes/     Markdown file I/O, ripgrep integration, fzf integration
  web/       HTTP server, routes, embedded static assets (go:embed)
  config/    Notes dir, API key, editor, port
```

- **Single binary**: web assets embedded via `go:embed` in `web/static/`
- **CLI framework**: `github.com/spf13/cobra`
- **Web stack**: `net/http` + `html/template` + HTMX + Pico CSS + CodeMirror
- **Markdown rendering**: `github.com/yuin/goldmark`
- **AI flow (`n add`)**: raw input -> Claude API formats -> save as markdown with YAML frontmatter
- **AI flow (`n ask`)**: ripgrep keyword search -> read top matching files -> Claude API answers citing filenames -> stream response
- **Backup**: git auto-commit goroutine in `n serve`, auto-generated commit messages

## Workflow — Learning Project

This is a learning project. The goal is understanding, not speed. Follow this loop:
- **I propose next steps** — don't jump ahead or build things I haven't asked for
- **We brainstorm together** — discuss tradeoffs, alternatives, and *why* before *how*
- **Implement thoughtfully** — I should understand the full architecture and every decision before we write code
- When in doubt, explain the concept or pattern before using it
- Prefer small, incremental steps that I can reason about over large leaps

## Conventions

- Notes are flat files in `~/notes/` — no folder hierarchy, tags for organization
- Note naming: `YYYY-MM-DD-slug.md` (date-stamped) or `slug.md` (persistent)
- YAML frontmatter (title, date, tags) added by AI formatting
- Support `[[wiki-links]]` in web UI
- Bullet points over numbered lists in note content
- Guard clauses and early returns over nested if/else
