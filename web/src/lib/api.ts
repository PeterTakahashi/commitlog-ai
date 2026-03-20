import type { PaginatedTimeline, Session, Stats } from "./types";

const BASE = "/api";

export async function fetchTimeline(params: {
  agent?: string;
  page?: number;
  pageSize?: number;
  search?: string;
}): Promise<PaginatedTimeline> {
  const sp = new URLSearchParams();
  if (params.agent) sp.set("agent", params.agent);
  if (params.page) sp.set("page", String(params.page));
  if (params.pageSize) sp.set("page_size", String(params.pageSize));
  if (params.search) sp.set("q", params.search);

  const qs = sp.toString();
  const res = await fetch(`${BASE}/timeline${qs ? `?${qs}` : ""}`);
  if (!res.ok) throw new Error(`Failed to fetch timeline: ${res.statusText}`);
  return res.json();
}

export async function fetchSession(
  id: string,
  opts?: { start?: number; end?: number },
): Promise<Session> {
  const sp = new URLSearchParams();
  if (opts?.start != null && opts?.end != null) {
    sp.set("start", String(opts.start));
    sp.set("end", String(opts.end));
  }
  const qs = sp.toString();
  const res = await fetch(`${BASE}/sessions/${id}${qs ? `?${qs}` : ""}`);
  if (!res.ok) throw new Error(`Failed to fetch session: ${res.statusText}`);
  return res.json();
}

export interface CommitWithDiff {
  hash: string;
  author: string;
  author_email: string;
  message: string;
  timestamp: string;
  files_changed: number;
  additions: number;
  deletions: number;
  changed_files?: string[];
  diff: string;
}

export async function fetchSessionCommits(
  sessionId: string,
): Promise<CommitWithDiff[]> {
  const res = await fetch(`${BASE}/sessions-commits/${sessionId}`);
  if (!res.ok)
    throw new Error(`Failed to fetch session commits: ${res.statusText}`);
  return res.json();
}

export async function fetchDiff(hash: string): Promise<string> {
  const res = await fetch(`${BASE}/commits/${hash}/diff`);
  if (!res.ok) throw new Error(`Failed to fetch diff: ${res.statusText}`);
  const data = await res.json();
  return data.diff;
}

export async function fetchStats(): Promise<Stats> {
  const res = await fetch(`${BASE}/stats`);
  if (!res.ok) throw new Error(`Failed to fetch stats: ${res.statusText}`);
  return res.json();
}

const avatarCache = new Map<string, string>();

export async function fetchAvatar(email: string): Promise<string> {
  if (avatarCache.has(email)) return avatarCache.get(email)!;
  try {
    const res = await fetch(
      `${BASE}/avatar?email=${encodeURIComponent(email)}`,
    );
    if (!res.ok) return "";
    const data = await res.json();
    avatarCache.set(email, data.avatar_url);
    return data.avatar_url;
  } catch {
    return "";
  }
}
