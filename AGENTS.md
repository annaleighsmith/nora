# CLAUDE.md

This file provides guidance to agents working in this repository.

## Project Overview

Nora is a self-hosted, terminal-first note-taking tool with a personality. It's a single Go binary that turns messy thoughts into clean markdown, then gives you an AI assistant (named by you, configurable personality) who actually *knows* your notes — searching, reading, remembering across sessions, and getting sharper the more you use it. Plain markdown files, flat directory, no lock-in, no cloud — just your notes, your terminal, and an AI that feels like yours.

This will be an open source GitHub project.

## Build & Run

```bash
go build -o nora .           # build binary
go run .                  # run directly
go test ./...             # run all tests
go test ./notes/...       # run tests for a package
go test -run TestName ./pkg/  # run a single test
```

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
- **AI flow (`n ask`)**: multi-turn sessions with 5 tools (search, read, list_tags, list_recent, note_index) -> Claude streams answers citing filenames -> memories saved between sessions
- **AI flow (`n import`)**: batch import with AI-generated frontmatter, inline tag extraction, mod-date preservation
- **AI flow (`n tags add --ai`)**: ripgrep candidates filtered by AI for relevance
- **Bot identity**: configurable name + personality in `[bot]` config, injected into system prompt
- **Token tracking**: per-model usage with cache-aware cost calculation
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

## CLI Design

- Follow unix conventions for flags and arg patterns (e.g. `-r` for recursive, `-f` for force, `-x` for exclude)
- Avoid flag collisions with well-known unix meanings (`-i` = interactive, `-v` = verbose, `-n` = dry-run)
- Model commands after familiar tools where applicable (`import` follows `cp` patterns, `tags` follows `git tag`)
- Short flags for common operations, long flags for less common ones
- Commands should work intuitively for anyone comfortable with a terminal

## Issue Tracking

This project uses **br** (beads_rust) for issue tracking. Run `br --help` for full usage.

```bash
br ready                                                            # what to work on next (open, unblocked, not deferred)
br show <id>                                                        # full issue details + dependencies
br q "title" -p 2 -t bug                                           # quick-file an issue (prints ID only)
br create "title" --description="..." -t bug|feature|task -p 0-4   # file an issue (verbose)
br update <id> --claim                                              # assign to self + mark in_progress
br close <id>                                                       # mark work done
br dep add <issue> <blocked-by>                                     # wire a dependency
br sync --flush-only                                                # export state to .beads/
git add .beads/ && git commit -m "Sync beads"                       # persist changes
```
