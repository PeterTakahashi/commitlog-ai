export interface Agent {
  tool: "claude_code" | "gemini_cli" | "codex_cli";
  model: string;
}

export interface ToolCall {
  tool: string;
  input: string;
  output?: string;
}

export interface TokenUsage {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
}

export interface Message {
  role: "human" | "assistant";
  content: string;
  timestamp: string;
  tool_calls?: ToolCall[];
  usage?: TokenUsage;
  model?: string;
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
  author_email: string;
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
  message_start_idx?: number;
  message_end_idx?: number;
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

export interface DiffStat {
  additions: number;
  deletions: number;
  files_changed: number;
  commits: number;
}

export interface TokenStat {
  input_tokens: number;
  output_tokens: number;
  cache_creation_input_tokens: number;
  cache_read_input_tokens: number;
}

export interface Stats {
  total_entries: number;
  total_sessions: number;
  total_messages: number;
  by_agent: Record<string, number>;
  diff_by_agent: Record<string, DiffStat>;
  token_by_agent: Record<string, TokenStat>;
  linked: number;
  commit_only: number;
  session_only: number;
}
