# aitrace - Development Guide for Claude Code

## Project Overview

aitrace is a Go CLI + React SPA that connects AI agent conversations (Claude Code, Gemini CLI, Codex CLI) to git commit history. Single binary, zero external dependencies.

## Tech Stack

- **Backend**: Go 1.25+, Cobra CLI framework, standard library only (no CGO, no database)
- **Frontend**: React 18 + TypeScript, Vite, Tailwind CSS v4, shadcn/ui, React Router
- **Integration**: React build is embedded into Go binary via `go:embed`

## Build Commands

```bash
# Full rebuild (always do this after frontend changes)
cd web && npm run build && cd ..
rm -rf internal/server/dist && cp -r web/dist internal/server/dist
go build -o bin/aitrace ./cmd/aitrace/

# Go only (backend changes, no frontend changes)
go build -o bin/aitrace ./cmd/aitrace/

# Frontend type check
cd web && npx tsc --noEmit

# Test the binary
./bin/aitrace status
./bin/aitrace parse
./bin/aitrace link
./bin/aitrace serve --no-browser
./bin/aitrace export --format markdown
```

## Project Structure

```
cmd/aitrace/
  main.go              Root cobra command
  cmd_parse.go         `aitrace parse` - runs all parsers, writes .aitrace/sessions.json
  cmd_link.go          `aitrace link` - matches sessions to git commits, writes .aitrace/timeline.json
  cmd_serve.go         `aitrace serve` - starts HTTP server with embedded React SPA
  cmd_export.go        `aitrace export` - exports as JSON or Markdown
  cmd_status.go        `aitrace status` - shows detected log sources

internal/
  model/
    session.go         Core types: Session, Agent, Message, ToolCall
    timeline.go        Core types: LinkedTimeline, TimelineEntry, GitCommit

  parser/
    parser.go          Parser interface (Name, Detect, Parse)
    claude.go          Claude Code parser (~/.claude/projects/<hash>/<uuid>.jsonl)
    gemini.go          Gemini CLI parser (~/.gemini/tmp/<hash>/chats/session-*.json)
    codex.go           Codex CLI parser (~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl)

  linker/
    git.go             Git CLI wrapper (git log, git diff --numstat, git diff)
    matcher.go         Timestamp-based matching algorithm with confidence scoring

  exporter/
    json.go            JSON export
    markdown.go        Markdown export (single file with all conversations)

  server/
    server.go          HTTP server: paginated /api/timeline, /api/sessions/:id, /api/commits/:hash/diff, /api/stats
    embed.go           go:embed for React dist files

web/
  src/
    lib/types.ts       TypeScript types matching Go model (Session, TimelineEntry, PaginatedTimeline, etc.)
    lib/api.ts         API client with pagination params
    pages/
      TimelinePage.tsx      Main view with infinite scroll, search, agent filter
      SessionDetailPage.tsx Split view: conversation + diff
      StatsPage.tsx         Stats dashboard
    components/
      TimelineList.tsx      Timeline entries grouped by date
      AgentBadge.tsx        Agent type badge (Claude=orange, Gemini=blue, Codex=green)
      ConfidenceDot.tsx     Match confidence indicator
      ConversationView.tsx  Chat-style message display
      ToolCallBlock.tsx     Expandable tool call display
      DiffViewer.tsx        GitHub-style diff viewer
```

## Key Architecture Decisions

- **No database**: JSON files in `.aitrace/` are loaded into memory at serve time. Server-side pagination via Go slices. This keeps `go install` working without CGO/SQLite.
- **Auto port fallback**: If default port 3000 is busy, server auto-selects a free port.
- **Dark mode**: Applied via `document.documentElement.classList.add("dark")` in App.tsx. Must be on `<html>`, not a wrapper `<div>`.
- **JSONL buffer size**: Claude/Codex parsers use 10MB scanner buffer for large log lines.
- **Codex user messages**: `response_item` with `role: "user"` contains system prompts. Real user input comes from `event_msg` with `type: "user_message"`.
- **go:embed workflow**: `web/dist/` must be copied to `internal/server/dist/` before `go build`. The `internal/server/dist/` directory should be in `.gitignore`.

## Agent Log Formats

### Claude Code (`~/.claude/projects/<project-hash>/<session-uuid>.jsonl`)
- JSONL, one JSON per line
- Key fields: `type` ("user"|"assistant"), `sessionId`, `timestamp`, `message.content` (array of content blocks), `message.model`, `isSidechain`
- Tool uses in assistant content blocks (`type: "tool_use"`), tool results in next user message (`type: "tool_result"`, matched by `tool_use_id`)

### Gemini CLI (`~/.gemini/tmp/<projectHash>/chats/session-*.json`)
- Single JSON file per session
- Root: `{sessionId, startTime, lastUpdated, messages: [...]}`
- Message types: "user" | "gemini"
- Tool calls inline in gemini messages: `toolCalls[].{name, args, result[].functionResponse.response.output}`

### Codex CLI (`~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`)
- JSONL with typed entries: `session_meta`, `response_item`, `event_msg`, `turn_context`
- `session_meta` has git info (commit_hash, branch, repository_url)
- `turn_context` has model name
- `event_msg.type: "user_message"` has actual user input
- `response_item.type: "function_call"` + `"function_call_output"` paired by `call_id`

## API Endpoints

```
GET /api/timeline?page=1&page_size=50&agent=claude_code&q=search
    → PaginatedTimeline {entries, git_repo, total, page, page_size, has_more}

GET /api/sessions/:id
    → Session (with full messages)

GET /api/commits/:hash/diff
    → {diff: "...full diff text..."}

GET /api/stats
    → {total_entries, total_sessions, total_messages, by_agent, linked, commit_only, session_only}
```

## Common Tasks

### Adding a new agent parser
1. Create `internal/parser/newagent.go` implementing the `Parser` interface
2. Add to `AllParsers()` in `internal/parser/parser.go`
3. No other changes needed - parse/status commands auto-discover parsers

### Changing the matching algorithm
Edit `internal/linker/matcher.go` → `computeConfidence()` function

### Adding a new API endpoint
1. Add handler in `internal/server/server.go`
2. Register in `Start()` method's `mux.HandleFunc()`
3. Update `web/src/lib/api.ts` and `web/src/lib/types.ts`
