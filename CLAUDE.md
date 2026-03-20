# commitlog-ai - Development Guide for Claude Code

## Project Overview

commitlog-ai is a Go CLI + React SPA that connects AI coding agent conversations (Claude Code, Gemini CLI, Codex CLI) to git commit history. Single binary, zero external dependencies.

## Tech Stack

- **Backend**: Go 1.25+, Cobra CLI framework, standard library only (no CGO, no database)
- **Frontend**: React 18 + TypeScript, Vite, Tailwind CSS v4, shadcn/ui, React Router
- **Integration**: React build is embedded into Go binary via `go:embed`

## Build Commands

```bash
# Full rebuild (always do this after frontend changes)
cd web && npm run build && cd ..
rm -rf internal/server/dist && cp -r web/dist internal/server/dist
go build -o bin/commitlog-ai ./cmd/commitlog-ai/

# Go only (backend changes, no frontend changes)
go build -o bin/commitlog-ai ./cmd/commitlog-ai/

# Frontend type check
cd web && npx tsc --noEmit

# Test the binary
./bin/commitlog-ai status
./bin/commitlog-ai parse
./bin/commitlog-ai link
./bin/commitlog-ai serve --no-browser
./bin/commitlog-ai serve --build --no-browser
./bin/commitlog-ai export --format markdown
```

## Project Structure

```
cmd/commitlog-ai/
  main.go              Root cobra command
  cmd_parse.go         `commitlog-ai parse` - runs parser, sanitizes secrets, writes .commitlog-ai/sessions.json (--force to bypass cache)
  cmd_link.go          `commitlog-ai link` - matches sessions to git commits, sanitizes, writes .commitlog-ai/timeline.json (--force to bypass cache)
  cmd_serve.go         `commitlog-ai serve` - starts HTTP server with embedded React SPA (--build for auto parse+link+rebuild)
  cmd_export.go        `commitlog-ai export` - exports as JSON or Markdown
  cmd_status.go        `commitlog-ai status` - shows detected log sources

internal/
  model/
    session.go         Core types: Session, Agent, Message, ToolCall
    timeline.go        Core types: LinkedTimeline, TimelineEntry, GitCommit (includes AuthorEmail)

  parser/
    parser.go          Parser interface (Name, Detect, Parse) + AllParsers() + ParserVersion constant
    claude.go          Claude Code parser (~/.claude/projects/<hash>/<uuid>.jsonl)
    gemini.go          Gemini CLI parser (~/.gemini/tmp/<project>/chats/session-*.json)
    codex.go           Codex CLI parser (~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl + legacy .json)

  linker/
    git.go             Git CLI wrapper (git log with %H%aI%an%ae%B using %x01 separator, git diff --numstat, git diff, GetHead)
    matcher.go         Timestamp-based matching algorithm with confidence scoring + session segmentation by commit

  builder/
    builder.go         Unified parse+link logic with caching for serve --build. Exports Build(projectDir) → Result

  cache/
    cache.go           Parse/link caching. ParseCache checks file size+mtime+parser version. LinkCache checks sessions.json mtime+git HEAD+parser version. Stored in .commitlog-ai/cache.json

  exporter/
    json.go            JSON export
    markdown.go        Markdown export (rune-aware truncation for multibyte safety)

  sanitizer/
    sanitizer.go       API key/secret detection and masking (OpenAI, Anthropic, AWS, Azure, GitHub, Bearer, etc.)

  server/
    server.go          HTTP server with sync.RWMutex for hot reload, ReloadData() method, paginated API endpoints
    embed.go           go:embed for React dist files

web/
  src/
    lib/types.ts       TypeScript types matching Go model (Session, TimelineEntry, GitCommit with author_email, etc.)
    lib/api.ts         API client with pagination params, avatar cache, debounced search
    pages/
      TimelinePage.tsx      Main view with infinite scroll, search (commit hash + message), agent filter
      SessionDetailPage.tsx Split view: conversation segment + diff, passes author name from URL params
      SessionFullPage.tsx   Full session view: complete conversation + all commits with collapsible diffs
      StatsPage.tsx         Stats dashboard
    components/
      TimelineList.tsx      Timeline entries grouped by date, with author avatar, multi-line commit messages
      AuthorAvatar.tsx      GitHub avatar via /api/avatar (Gravatar fallback) + initials fallback
      AgentBadge.tsx        Agent type badge (Claude=orange)
      ConfidenceDot.tsx     Match confidence indicator
      ConversationView.tsx  Chat-style display: human label from commit author, tool approval detection, empty message filtering
      ToolCallBlock.tsx     Expandable tool call display
      DiffViewer.tsx        GitHub-style diff viewer
```

## Key Architecture Decisions

- **No database**: JSON files in `.commitlog-ai/` are loaded into memory at serve time. Server-side pagination via Go slices. This keeps `go install` working without CGO/SQLite.
- **Default port 3100**: Changed from 3000 to avoid conflicts with common dev servers. Auto-fallback via `net.Listen("tcp", "localhost:0")` if busy.
- **Caching**: File modification time + size for parse cache, git HEAD hash for link cache. Cache metadata in `.commitlog-ai/cache.json`. Parser version (`ParserVersion` in `parser.go`) change invalidates all caches. Parse invalidation also clears link cache.
- **serve --build**: Runs `builder.Build()` at startup, then polls `git rev-parse HEAD` every 2 seconds in a goroutine. On HEAD change: rebuild + `server.ReloadData()`. Uses `sync.RWMutex` so API handlers continue serving during reload.
- **Git log format**: Uses `%B` (full commit body) instead of `%s` (subject only) with `%x01` as record separator to support multi-line commit messages.
- **Session segmentation**: Long sessions are split by commit boundaries with minimum 4 messages per segment. Segments can overlap backwards for context.
- **Dark mode**: Applied via `document.documentElement.classList.add("dark")` in App.tsx. Must be on `<html>`, not a wrapper `<div>`.
- **JSONL buffer size**: Claude parser uses 10MB scanner buffer for large log lines.
- **Rune-aware truncation**: Markdown export truncates by `[]rune` count, not byte count, to avoid splitting multibyte UTF-8 characters.
- **Secret sanitization**: Applied in both `cmd_parse.go` and `cmd_link.go` before writing any file. Important: when making new slices from old slices, save the original reference before `make()` to avoid iterating over zeroed data.
- **go:embed workflow**: `web/dist/` must be copied to `internal/server/dist/` before `go build`. The `internal/server/dist/` directory is in `.gitignore`.
- **Avatar resolution**: Server tries GitHub user search API first, falls back to Gravatar (identicon). Results cached in `sync.Map`.
- **Author name in conversation**: Passed via URL search param `?author=Name` from TimelineList to SessionDetailPage to ConversationView.
- **Tool approval messages**: Empty human messages following an assistant tool call are displayed as compact "approved {tool_name}" indicators. Other empty messages are hidden.

## Agent Log Format

### Claude Code (`~/.claude/projects/<project-hash>/<session-uuid>.jsonl`)
- JSONL, one JSON per line
- Key fields: `type` ("user"|"assistant"), `sessionId`, `timestamp`, `message.content` (array of content blocks), `message.model`, `isSidechain`
- Tool uses in assistant content blocks (`type: "tool_use"`), tool results in next user message (`type: "tool_result"`, matched by `tool_use_id`)
- Skip entries with `isSidechain: true` (subagent logs)

### Gemini CLI (`~/.gemini/tmp/<project-dir-or-hash>/chats/session-*.json`)
- Single JSON file per session (not JSONL)
- Top-level: `sessionId`, `startTime`, `lastUpdated`, `messages[]`, `kind` ("main" = keep, others = skip)
- `.project_root` file in parent dir contains absolute project path for matching
- Messages: `type` ("user"|"gemini"), `content` (string for gemini, array of `{text}` for user), `model`, `tokens`, `toolCalls[]`
- Tool calls: `name`, `args` (JSON), `result[].functionResponse.response.output`

### Codex CLI (`~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`)
- JSONL format (new) or single JSON (legacy `rollout-*.json`)
- JSONL entry types: `session_meta` (id, cwd, git info), `turn_context` (model), `event_msg` (user_message/agent_message), `response_item` (function_call/function_call_output)
- User input comes from `event_msg` with `type: "user_message"`, NOT from `response_item` with `role: "user"` (those are system prompts)
- Tool calls: `response_item` with `type: "function_call"` (name, arguments, call_id) paired with `type: "function_call_output"` (call_id, output)
- Legacy JSON: `{session: {id, timestamp}, items: [{type, role, content, action, call_id, output}]}`

## API Endpoints

```
GET /api/timeline?page=1&page_size=50&agent=claude_code&q=search
    → PaginatedTimeline {entries, git_repo, total, page, page_size, has_more}
    Search matches commit hash and commit message

GET /api/sessions/:id?start=0&end=10
    → Session (with full or sliced messages for segmented views)

GET /api/sessions-commits/:id
    → [{hash, author, message, timestamp, diff, ...}] (all commits linked to session with diffs)

GET /api/commits/:hash/diff
    → {diff: "...full diff text..."}

GET /api/stats
    → {total_entries, total_sessions, total_messages, by_agent, linked, commit_only, session_only}

GET /api/avatar?email=user@example.com
    → {avatar_url: "https://..."}
```

## Common Tasks

### Adding a new agent parser
1. Create `internal/parser/newagent.go` implementing the `Parser` interface
2. Add to `AllParsers()` in `internal/parser/parser.go`
3. Bump `ParserVersion` in `internal/parser/parser.go` to invalidate caches
4. No other changes needed - parse/status commands auto-discover parsers

### Changing the matching algorithm
Edit `internal/linker/matcher.go` → `computeConfidence()` function

### Adding a new API endpoint
1. Add handler in `internal/server/server.go` (use `s.mu.RLock()/RUnlock()` for data access)
2. Register in `Start()` method's `mux.HandleFunc()`
3. Update `web/src/lib/api.ts` and `web/src/lib/types.ts`

### Adding new secret patterns
Add regex to `secretPatterns` slice in `internal/sanitizer/sanitizer.go`
