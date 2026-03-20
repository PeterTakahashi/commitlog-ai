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

export async function fetchSession(id: string): Promise<Session> {
  const res = await fetch(`${BASE}/sessions/${id}`);
  if (!res.ok) throw new Error(`Failed to fetch session: ${res.statusText}`);
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
