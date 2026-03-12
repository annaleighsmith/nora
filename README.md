# Nora

A self-hosted, terminal-first note-taking tool with AI built in. Plain markdown files, flat directory, no lock-in, no cloud.

Born out of wanting to leave Obsidian behind, quickly fire off markdown into a vault, and launch AI conversations from my notes — without plugins, sync services, or electron apps. Just markdown files and a terminal.

You jot something down in neovim, and Nora can organize it, format it, or synthesize it later. You ask questions — Nora searches your notes, reads them, and gives you real answers with citations. She remembers things between sessions and gets sharper the more you use her.

## Setup

### Dependencies

You'll need these on your `PATH`:

- [Go](https://go.dev/) (to build)
- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) — search
- [fzf](https://github.com/junegunn/fzf) — fuzzy selection
- `$EDITOR` (defaults to neovim) — note editing

### Install

```bash
git clone https://github.com/annaleighsmith/nora.git
cd nora
./install.sh
```

### API key

Nora uses the Anthropic API. Set your key:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

Add that to your `.zshrc` / `.bashrc` to persist it.

### First run

```bash
nora setup
```

This prompts for your notes directory (default `~/notes/`) and creates the config at `~/.config/nora/config.toml`.

Run `nora --help` to see all commands and options.

### Phone access

Right now I use [Tailscale](https://tailscale.com/) + [Terminus](https://termius.com/) to SSH into my machine and run Nora from my phone. Works great for quick lookups and `ask` sessions. A proper web UI (`nora serve`) is in progress for a more mobile-friendly experience.

## Heads up

- **Token usage is not fully optimized yet.** Multi-turn `ask` sessions can get expensive with large vaults. Read budgets help, but there's more work to do here.
- **Memory is user-managed for now.** Nora saves memories between `ask` sessions, but there's no UI to review or prune them yet. You can manually edit `~/.config/nora/memories.md`.
- **Security is straightforward.** Nora doesn't make any network calls with your notes other than to your configured AI provider. Everything else is local. Local model support is a planned feature for full privacy.

## Planned features

- Multi-provider support (OpenAI, Ollama, local models)
- Web UI (`nora serve`) with HTMX + Pico CSS + CodeMirror
- `[[wiki-links]]` support

## License

MIT
