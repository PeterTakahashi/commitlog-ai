export interface Agent {
  tool: "claude_code" | "gemini_cli" | "codex_cli";
  model: string;
}

export interface ToolCall {
  tool: string;
  input: string;
  output?: string;
}

export interface Message {
  role: "human" | "assistant";
  content: string;
  timestamp: string;
  tool_calls?: ToolCall[];
}

export interface Session {
  id: string;
  agent: Agent;
  project: string;
  cwd: string;
  git_branch?: string;
  started_at: string;
  ended_at: string;
  messages: Message[] | null;
}

export interface GitCommit {
  hash: string;
  author: string;
  message: string;
  timestamp: string;
  files_changed: number;
  additions: number;
  deletions: number;
  changed_files?: string[];
}

export interface TimelineEntry {
  commit?: GitCommit;
  session?: Session;
  link_confidence: number;
}

export interface LinkedTimeline {
  entries: TimelineEntry[];
  git_repo: string;
}

export interface PaginatedTimeline {
  entries: TimelineEntry[];
  git_repo: string;
  total: number;
  page: number;
  page_size: number;
  has_more: boolean;
}

export interface Stats {
  total_entries: number;
  total_sessions: number;
  total_messages: number;
  by_agent: Record<string, number>;
  linked: number;
  commit_only: number;
  session_only: number;
}
