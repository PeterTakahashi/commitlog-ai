# commitlog-ai

> See the prompts behind every git commit.

**commitlog-ai** connects your AI coding agent conversations to your git history, giving your team a complete picture of AI-assisted development.

## The Problem

You use Claude Code daily. But `git log` only shows *what* changed, not *why*, not *which prompt*, not *which model*. Agent logs are buried in hidden directories with no link back to git.

## The Solution

commitlog-ai reads agent logs (Claude Code, Gemini CLI, Codex CLI), converts them into a unified format, matches them to git commits by timestamp, and serves a web UI to browse it all. API keys and secrets found in logs are automatically masked before output.

```
Agent Logs ──▶ commitlog-ai parse ──▶ commitlog-ai link ──▶ commitlog-ai serve
(Claude/Gemini/Codex)                                    │
                                                 localhost:3100
                                        Timeline + Conversations + Diffs
```

## Quick Start

```bash
go install github.com/anthropics/commitlog-ai@latest

cd your-project

# Option 1: All-in-one (recommended)
commitlog-ai serve --build    # Parse + link + serve, auto-rebuilds on new commits

# Option 2: Step by step
commitlog-ai parse            # Read agent logs → unified format
commitlog-ai link             # Match sessions to git commits
commitlog-ai serve            # Open web UI at localhost:3100
```

## Supported Agents

| Agent | Status | Minimum Version | Log Location |
|-------|--------|----------------|-------------|
| Claude Code | Supported | 1.0.0+ | `~/.claude/projects/` |
| Gemini CLI | Supported | 0.1.0+ | `~/.gemini/tmp/<project>/chats/` |
| Codex CLI | Supported | 0.1.0+ | `~/.codex/sessions/` |

> **Note**: commitlog-ai reads the local log files that each agent writes to disk. If your agent version is too old and uses a different log format, parsing may fail. The versions above are the earliest known to produce compatible logs. Tested with Claude Code 2.1.79, Gemini CLI 0.34.0, and Codex CLI 0.116.0.

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

### `commitlog-ai status`

Show detected log sources and counts for the current project.

```
$ commitlog-ai status
Project: /Users/you/dev/myproject

  claude_code   3 log file(s)
  gemini_cli    1 log file(s)
  codex_cli     2 log file(s)
```

### `commitlog-ai parse`

Parse all detected agent logs into a unified JSON format. Secrets are automatically masked. Output is written to `.commitlog-ai/sessions.json`. Results are cached and only re-parsed when source files change or the parser version is updated.

```
$ commitlog-ai parse
[claude_code] Found 3 log file(s)
  Session a1b2c3d4: 42 messages (09:15:30 to 10:22:45)
  Session e5f6g7h8: 18 messages (14:00:12 to 14:35:20)

Parsed 2 session(s) → .commitlog-ai/sessions.json
```

Options:
- `--force` — Ignore cache and re-parse all logs

### `commitlog-ai link`

Match parsed sessions to git commits using timestamp-based heuristics. Output is written to `.commitlog-ai/timeline.json`. Results are cached and only re-linked when sessions.json changes or new commits are detected.

```
$ commitlog-ai link
Found 2 session(s) and 28 commit(s)
Linked 2 pair(s), 28 total entries → .commitlog-ai/timeline.json
```

Options:
- `--force` — Ignore cache and re-link

### `commitlog-ai serve`

Start a local web server to browse the linked timeline. If the default port is in use, an available port is automatically selected.

```
$ commitlog-ai serve
commitlog-ai server running at http://localhost:3100
  28 timeline entries, 2 sessions
```

Options:
- `--build` — Run parse+link before serving and auto-rebuild on new git commits
- `--port <number>` — Server port (default: 3100, auto-fallback if busy)
- `--no-browser` — Don't open browser automatically

### `commitlog-ai export`

Export the linked timeline as JSON or Markdown.

```bash
# JSON bundle
commitlog-ai export --format json
Exported → .commitlog-ai/output/timeline.json

# Markdown report (single file with all conversations)
commitlog-ai export --format markdown
Exported → .commitlog-ai/output/timeline.md
```

Options:
- `--format json` — JSON bundle (default)
- `--format markdown` — Single Markdown file with summary, commit details, and full conversations in collapsible sections

## How Matching Works

commitlog-ai links sessions to commits using a confidence-scored algorithm:

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
- **Stats** — Dashboard showing session counts by agent, diff and token stats by agent, and link status breakdown.

### Screenshots

| Timeline | Session Detail |
|----------|---------------|
| ![Timeline](docs/img/timeline.png) | ![Session Detail](docs/img/session-detail.png) |

| Full Session | Stats |
|-------------|-------|
| ![Full Session](docs/img/session-full.png) | ![Stats](docs/img/stats.png) |

## Architecture

- **Go CLI** — Single binary, zero external dependencies (no database, no Docker)
- **React SPA** — Built with Vite + TypeScript + Tailwind CSS v4 + shadcn/ui, embedded into the Go binary via `go:embed`
- **JSON-based** — All data stored as JSON files in `.commitlog-ai/`, portable and git-friendly. Loaded into memory at serve time with server-side pagination.
- **Caching** — File modification time + size for parse cache, git HEAD hash for link cache. Cache metadata stored in `.commitlog-ai/cache.json`. Parser version changes invalidate all caches.
- **Hot Reload** — `serve --build` uses `sync.RWMutex` to safely swap data while API handlers continue serving requests
- **Secret Sanitizer** — Regex-based detection and masking of API keys, tokens, and credentials before any file is written

## Development

```bash
# Build everything (web + Go)
cd web && npm run build && cd ..
rm -rf internal/server/dist && cp -r web/dist internal/server/dist
go build -o bin/commitlog-ai ./cmd/commitlog-ai/

# Development: run Vite dev server + Go API separately
cd web && npm run dev          # Vite on :5173 (proxies /api to :3100)
go run ./cmd/commitlog-ai/ serve    # Go API on :3100
```

### Project Structure

```
cmd/commitlog-ai/           CLI entry point and subcommands
internal/
  model/               Unified data types (Session, Message, Timeline)
  parser/              Log parsers (Claude Code, Gemini CLI, Codex CLI)
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
