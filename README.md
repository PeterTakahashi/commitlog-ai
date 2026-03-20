# aitrace

> See the prompts behind every git commit.

**aitrace** connects your Claude Code conversations to your git history, giving your team a complete picture of AI-assisted development.

## The Problem

You use Claude Code daily. But `git log` only shows *what* changed, not *why*, not *which prompt*, not *which model*. Agent logs are buried in hidden directories with no link back to git.

## The Solution

aitrace reads Claude Code logs, converts them into a unified format, matches them to git commits by timestamp, and serves a web UI to browse it all. API keys and secrets found in logs are automatically masked before output.

```
Claude Code Logs ──▶ aitrace parse ──▶ aitrace link ──▶ aitrace serve
                                                             │
                                                    localhost:3100
                                           Timeline + Conversations + Diffs
```

## Quick Start

```bash
go install github.com/anthropics/aitrace@latest

cd your-project

# Option 1: All-in-one (recommended)
aitrace serve --build    # Parse + link + serve, auto-rebuilds on new commits

# Option 2: Step by step
aitrace parse            # Read Claude Code logs → unified format
aitrace link             # Match sessions to git commits
aitrace serve            # Open web UI at localhost:3100
```

## Supported Agents

| Agent | Status | Log Location |
|-------|--------|-------------|
| Claude Code | Supported | `~/.claude/projects/` |

## Features

- **`serve --build`** — One command does everything: parse, link, serve, and auto-rebuild when new git commits are detected (2-second polling)
- **Caching** — Parse and link results are cached; only rebuilds when source files change, new commits appear, or parser version is updated. Use `--force` to bypass.
- **API Key Masking** — Secrets (OpenAI, Anthropic, AWS, Azure, GitHub tokens, etc.) are automatically detected and masked in all output files
- **Git Author Info** — Each commit shows author name, email, and GitHub profile icon in the web UI
- **Full Session View** — Browse the complete conversation for any session, with all code changes across every linked commit
- **Commit Hash Search** — Search timeline by commit hash or commit message with real-time filtering
- **Auto Port Fallback** — If port 3100 is busy, an available port is automatically selected
- **Server-side Pagination** — Handles repositories with thousands of commits efficiently
- **Markdown Export** — Export the full timeline as a single Markdown file

## Commands

### `aitrace status`

Show detected log sources and counts for the current project.

```
$ aitrace status
Project: /Users/you/dev/myproject

  claude_code   3 log file(s)
```

### `aitrace parse`

Parse all detected agent logs into a unified JSON format. Secrets are automatically masked. Output is written to `.aitrace/sessions.json`. Results are cached and only re-parsed when source files change or the parser version is updated.

```
$ aitrace parse
[claude_code] Found 3 log file(s)
  Session a1b2c3d4: 42 messages (09:15:30 to 10:22:45)
  Session e5f6g7h8: 18 messages (14:00:12 to 14:35:20)

Parsed 2 session(s) → .aitrace/sessions.json
```

Options:
- `--force` — Ignore cache and re-parse all logs

### `aitrace link`

Match parsed sessions to git commits using timestamp-based heuristics. Output is written to `.aitrace/timeline.json`. Results are cached and only re-linked when sessions.json changes or new commits are detected.

```
$ aitrace link
Found 2 session(s) and 28 commit(s)
Linked 2 pair(s), 28 total entries → .aitrace/timeline.json
```

Options:
- `--force` — Ignore cache and re-link

### `aitrace serve`

Start a local web server to browse the linked timeline. If the default port is in use, an available port is automatically selected.

```
$ aitrace serve
aitrace server running at http://localhost:3100
  28 timeline entries, 2 sessions
```

Options:
- `--build` — Run parse+link before serving and auto-rebuild on new git commits
- `--port <number>` — Server port (default: 3100, auto-fallback if busy)
- `--no-browser` — Don't open browser automatically

### `aitrace export`

Export the linked timeline as JSON or Markdown.

```bash
# JSON bundle
aitrace export --format json
Exported → .aitrace/output/timeline.json

# Markdown report (single file with all conversations)
aitrace export --format markdown
Exported → .aitrace/output/timeline.md
```

Options:
- `--format json` — JSON bundle (default)
- `--format markdown` — Single Markdown file with summary, commit details, and full conversations in collapsible sections

## How Matching Works

aitrace links sessions to commits using a confidence-scored algorithm:

1. **Time overlap** — Commit timestamp falls within session time range → 90% confidence
2. **Post-session commit** — Commit within 5 minutes after session ends → 70% confidence
3. **Pre-session commit** — Commit within 5 minutes before session starts → 50% confidence
4. **File overlap bonus** — Files touched by tool calls match commit's changed files → +10%
5. **Branch bonus** — Session's git branch matches → +5%

Unmatched commits and sessions are shown as standalone entries.

## Web UI

The web viewer provides four views:

- **Timeline** — A git-log-style list with infinite scroll, server-side pagination, agent filter, and search by commit hash or message. Each entry shows the commit author's GitHub avatar, name, and file change stats.
- **Session Detail** — Split view with the conversation segment on the left (showing the commit author's name instead of "You") and the git diff on the right. Tool approval messages are shown inline with the approved tool name.
- **Full Session** — Complete conversation for an entire session with a separate tab showing all code changes across every linked commit with collapsible diffs.
- **Stats** — Dashboard showing session counts by agent, link status breakdown, and message totals

## Architecture

- **Go CLI** — Single binary, zero external dependencies (no database, no Docker)
- **React SPA** — Built with Vite + TypeScript + Tailwind CSS v4 + shadcn/ui, embedded into the Go binary via `go:embed`
- **JSON-based** — All data stored as JSON files in `.aitrace/`, portable and git-friendly. Loaded into memory at serve time with server-side pagination.
- **Caching** — File modification time + size for parse cache, git HEAD hash for link cache. Cache metadata stored in `.aitrace/cache.json`. Parser version changes invalidate all caches.
- **Hot Reload** — `serve --build` uses `sync.RWMutex` to safely swap data while API handlers continue serving requests
- **Secret Sanitizer** — Regex-based detection and masking of API keys, tokens, and credentials before any file is written

## Development

```bash
# Build everything (web + Go)
cd web && npm run build && cd ..
rm -rf internal/server/dist && cp -r web/dist internal/server/dist
go build -o bin/aitrace ./cmd/aitrace/

# Development: run Vite dev server + Go API separately
cd web && npm run dev          # Vite on :5173 (proxies /api to :3100)
go run ./cmd/aitrace/ serve    # Go API on :3100
```

### Project Structure

```
cmd/aitrace/           CLI entry point and subcommands
internal/
  model/               Unified data types (Session, Message, Timeline)
  parser/              Claude Code log parser
  linker/              Git operations and timestamp-based matching
  builder/             Unified parse+link logic for serve --build
  cache/               Parse/link caching with parser version invalidation
  exporter/            JSON and Markdown export
  sanitizer/           API key and secret masking
  server/              HTTP server with embedded React SPA + paginated API
web/                   React viewer (Vite + TypeScript + Tailwind + shadcn/ui)
```

## License

MIT
