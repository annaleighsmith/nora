# Design: Encrypted Directories

**Status:** Exploration / RFC
**Issue:** nora-rlk

## Problem Statement

### Why encrypted directories?

Some notes contain sensitive content — diaries, credentials, medical info, financial details. Currently all notes sit as plaintext `.md` files in `~/notes/`. Anyone with filesystem access can read everything.

### Threat model

- **Laptop theft** — attacker has disk access, possibly bypassing OS login
- **Shared machines** — other users or processes can read `~/notes/`
- **Cloud backup exposure** — synced to Dropbox/iCloud/git remote, readable by the service or anyone who compromises the account

### What's protected

- Note content on disk (body + frontmatter)
- In-memory plaintext during active use is acceptable — this isn't about runtime memory attacks

## Current Architecture

### How notes work today

- Flat `.md` files in a single directory (`~/notes/`)
- YAML frontmatter (title, date, tags) parsed on read by `utils/frontmatter.go`
- Read/written via `os.ReadFile` / `os.WriteFile` scattered across the codebase

### File I/O surface area

~69 direct file I/O calls across 17 Go files. The hotspots:

| Package | File | I/O calls | What it does |
|---------|------|-----------|--------------|
| `utils` | `notes.go` | ~15 | Core note read/write, first lines, content helpers |
| `utils` | `tags.go` | ~8 | Tag listing, adding, finding untagged |
| `utils` | `pick.go` | ~6 | fzf integration, note listing, file reads for preview |
| `utils` | `frontmatter.go` | ~4 | Frontmatter parsing and fixing |
| `utils` | `frame.go` | ~3 | Search result framing reads files for metadata |
| `ai` | `tools.go` | ~8 | AI tool handlers (read_note, search, list_recent) |
| `ai` | `session.go` | ~4 | Memory load/save, session state |
| `cmd` | `import.go` | ~8 | Batch import with file walking, reading, writing |
| `cmd` | `add.go` | ~3 | Write new notes |
| `cmd` | `fix.go` | ~4 | Frontmatter and naming fixes |
| `cmd` | `delete.go` | ~2 | File deletion/archival |
| `cmd` | `search.go` | ~2 | Search output formatting |
| `web` | routes | ~2 | Web UI note serving |

### Ripgrep dependency

- 3 ripgrep shellout sites (`utils/notes.go` search, `utils/tags.go` tag finding, `utils/pick.go` tag scoping)
- Ripgrep reads files directly from disk — cannot search encrypted content
- This is the single biggest architectural blocker

### Frontmatter parsing

- Tags, dates, and titles are extracted from YAML frontmatter inside the `.md` file
- Every operation that filters/sorts/displays notes needs to parse frontmatter
- Requires plaintext access to the file header at minimum

## Approach Options

### Option A: File-level transparent encryption (decrypt-before-search)

- Encrypt each `.md` file individually
- Decrypt to temp/memory on read, re-encrypt on write
- All existing code works against plaintext in memory — minimal logic changes
- **Ripgrep must be replaced**: can't search encrypted files, so every search decrypts all files first or uses a Go-native search loop

**Pros:**
- Simplest mental model — "files are encrypted, that's it"
- No index to keep in sync
- Every file is fully protected

**Cons:**
- Search decrypts every file every time — O(n) decrypt for every query
- Performance degrades linearly with vault size
- Ripgrep replacement is significant work
- Preview/fzf integration needs rethinking

### Option B: Plaintext metadata index + file-level encryption

- Keep an unencrypted index file with frontmatter fields (title, date, tags, filename)
- Encrypt note bodies (everything after frontmatter, or the whole file)
- Metadata queries (list tags, filter by date, sort) use the index — no decryption needed
- Full-text search decrypts only matching files (or all, with caching)

**Pros:**
- Fast metadata operations — most commands don't need to decrypt
- Smaller decrypt footprint for common workflows
- Index is small, easy to rebuild from decrypted files
- Incremental: can encrypt per-directory, not whole vault

**Cons:**
- Index must stay in sync — risk of drift on crashes or external edits
- Index leaks metadata (titles, tags, dates) — weaker protection
- Two data sources to reason about
- Ripgrep still can't search encrypted bodies (need Go-native for full-text)

### Option C: Replace ripgrep with Go-native search

- Implement full search in Go, reading and decrypting files directly
- Could be combined with Option A or B
- Opens door to richer search features (fuzzy, semantic, weighted)

**Pros:**
- Removes external dependency entirely
- Full control over search behavior
- Can handle encrypted content natively

**Cons:**
- Highest implementation effort
- Ripgrep is fast — Go implementation likely slower for large vaults
- Loses ripgrep's battle-tested edge cases (binary detection, encoding handling, etc.)
- Could be done incrementally as part of A or B

## Recommended Path

**Option B (plaintext index + file-level encryption)** with a phased approach:

### Phase 1: Abstraction layer (`notestore`)
- Introduce a `notestore` package that wraps all file I/O
- All commands read/write through `notestore` instead of direct `os.ReadFile`/`os.WriteFile`
- No encryption yet — just the indirection layer
- This is the highest-effort phase but prerequisite for everything else

### Phase 2: File-level encryption
- `notestore` gains encrypt-on-write, decrypt-on-read capability
- Key derived from passphrase, cached for session duration
- Encrypted files get `.md.age` extension (or similar)
- Unencrypted notes continue to work alongside encrypted ones

### Phase 3: Metadata index
- `notestore` maintains a plaintext index of frontmatter fields
- Index rebuilt on `notestore.Reindex()` or incrementally on write
- Tag listing, date filtering, note listing use index instead of reading files

### Phase 4: Encrypted search
- Go-native search for encrypted directories (replaces ripgrep for those dirs)
- Ripgrep still used for unencrypted directories (fast path)
- Decrypt + search with result caching for repeated queries

## Open Questions

### Key management UX
- Passphrase prompt on first access each session?
- OS keyring integration (libsecret on Linux, Keychain on macOS)?
- Session caching duration — until process exits? Timed expiry?
- Multiple passphrases for different encrypted directories?

### Per-directory vs whole-vault
- Per-directory is more flexible (encrypt only `~/notes/private/`)
- Whole-vault is simpler to reason about
- Per-directory needs clear UX for "this dir is locked" vs "this dir is open"

### Git integration
- Encrypted files are binary blobs — no meaningful diffs
- `git log` and `git blame` become useless for encrypted notes
- Auto-commit (from `nora serve`) would commit encrypted blobs
- Could decrypt for diff, but that leaks content in git output

### Encryption library
- **age** — modern, simple API, good Go library (`filippo.io/age`), recommended
- **GPG** — established, complex, requires gpg binary on PATH
- **libsodium** — low-level, fast, but more implementation work
- age is the likely winner: simple key management, no external dependencies, actively maintained

### AI features
- Encrypted notes must be decrypted before sending to Claude API
- `nora ask` and `nora manage` read note content — need seamless decrypt
- Token tracking and memory files — encrypt those too?

### Search performance
- 100 notes: decrypt-all is fine (<1s)
- 1000+ notes: decrypt-all becomes noticeable
- Caching decrypted content in memory helps but increases memory footprint
- Index-first filtering reduces decrypt count for most queries

## Blast Radius Summary

Files that need changes for the `notestore` abstraction (Phase 1):

- **`utils/notes.go`** — All note I/O functions become `notestore` methods
- **`utils/tags.go`** — `ListTags`, `FindUntagged`, `AddTag` use notestore
- **`utils/pick.go`** — `listNotes`, `Pick`, file reads for preview
- **`utils/frontmatter.go`** — Parsing moves inside notestore or becomes a helper
- **`utils/frame.go`** — `FrameSearchResults` reads files for metadata
- **`utils/filename.go`** — Filename generation stays pure, but callers change
- **`ai/tools.go`** — Tool handlers (`read_note`, `search`, `list_recent`, `list_tags`) use notestore
- **`ai/session.go`** — Memory load/save, preloaded files
- **`cmd/add.go`** — `formatAndSave` writes through notestore
- **`cmd/import.go`** — Batch write through notestore
- **`cmd/delete.go`** — Delete/archive through notestore
- **`cmd/fix.go`** — Read + rewrite through notestore
- **`cmd/search.go`** — Search invocation changes
- **`cmd/ask.go`** — File-scoped mode reads through notestore
- **`cmd/manage.go`** — Write tools go through notestore
- **`web/`** — Route handlers read through notestore

The `notestore` abstraction is the critical prerequisite — once that's in place, encryption becomes a configuration option rather than a rewrite.
